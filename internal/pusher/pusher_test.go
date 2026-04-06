package pusher

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoHTTPSURL(t *testing.T) {
	got := repoHTTPSURL("owner/repo", "mytoken")
	want := "https://x-access-token:mytoken@github.com/owner/repo.git"
	if got != want {
		t.Errorf("repoHTTPSURL = %q, want %q", got, want)
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create a small directory tree in src.
	files := map[string]string{
		"a.txt":        "hello",
		"sub/b.txt":    "world",
		"sub/sub/c.go": "package main",
	}
	for rel, content := range files {
		abs := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for rel, want := range files {
		data, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing file %s: %v", rel, err)
			continue
		}
		if string(data) != want {
			t.Errorf("file %s: got %q, want %q", rel, string(data), want)
		}
	}
}

func TestEnsureGoMod_Creates(t *testing.T) {
	dir := t.TempDir()
	if err := ensureGoMod(dir, "github.com/org/svc-pb"); err != nil {
		t.Fatalf("ensureGoMod: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	prefix := "module github.com/org/svc-pb"
	if len(content) < len(prefix) || content[:len(prefix)] != prefix {
		t.Errorf("unexpected go.mod content: %q", content)
	}
}

func TestEnsureGoMod_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	existing := "module github.com/org/other\n\ngo 1.20\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGoMod(dir, "github.com/org/svc-pb"); err != nil {
		t.Fatalf("ensureGoMod: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Errorf("go.mod was overwritten: got %q", string(data))
	}
}

// ── GitHub API helpers ────────────────────────────────────────────────────────

// newTestGitHubServer builds a minimal httptest server that stubs the three
// GitHub API endpoints used by ensureRepoExists / repoExists / ownerIsOrg.
//
// repoStatus    – HTTP status returned for GET /repos/{owner}/{repo}
// orgStatus     – HTTP status returned for GET /orgs/{owner}
// createStatus  – HTTP status returned for POST /orgs/{owner}/repos or
//
//	POST /user/repos
func newTestGitHubServer(t *testing.T, repoStatus, orgStatus, createStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repos/"):
			w.WriteHeader(repoStatus)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/orgs/"):
			w.WriteHeader(orgStatus)
		case r.Method == http.MethodPost:
			w.WriteHeader(createStatus)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// patchHTTPClient temporarily replaces http.DefaultClient for the duration of
// the test and restores it afterwards.
func patchHTTPClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := http.DefaultClient
	http.DefaultClient = srv.Client()
	t.Cleanup(func() { http.DefaultClient = orig })
}

// patchAPIBase rewrites the hard-coded "https://api.github.com" URLs used by
// the helpers to point at the test server instead.  It does so by monkey-
// patching the package-level variable apiBase.
func withAPIBase(t *testing.T, base string) {
	t.Helper()
	orig := apiBase
	apiBase = base
	t.Cleanup(func() { apiBase = orig })
}

func TestRepoExists_True(t *testing.T) {
	srv := newTestGitHubServer(t, http.StatusOK, 0, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	got, err := repoExists("owner", "repo", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected repoExists to return true")
	}
}

func TestRepoExists_False(t *testing.T) {
	srv := newTestGitHubServer(t, http.StatusNotFound, 0, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	got, err := repoExists("owner", "repo", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected repoExists to return false")
	}
}

func TestRepoExists_UnexpectedStatus(t *testing.T) {
	srv := newTestGitHubServer(t, http.StatusInternalServerError, 0, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	_, err := repoExists("owner", "repo", "tok")
	if err == nil {
		t.Fatal("expected an error for unexpected HTTP status")
	}
}

func TestOwnerIsOrg_True(t *testing.T) {
	srv := newTestGitHubServer(t, 0, http.StatusOK, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	got, err := ownerIsOrg("myorg", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected ownerIsOrg to return true for org")
	}
}

func TestOwnerIsOrg_False(t *testing.T) {
	srv := newTestGitHubServer(t, 0, http.StatusNotFound, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	got, err := ownerIsOrg("alice", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected ownerIsOrg to return false for user")
	}
}

func TestEnsureRepoExists_AlreadyExists(t *testing.T) {
	srv := newTestGitHubServer(t, http.StatusOK, 0, 0)
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	// Should return nil without attempting to create.
	if err := ensureRepoExists("owner/repo", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureRepoExists_CreatesForUser(t *testing.T) {
	// repo 404, owner is not an org (404), creation succeeds (201).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repos/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/orgs/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/user/repos":
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	if err := ensureRepoExists("alice/new-repo", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureRepoExists_CreatesForOrg(t *testing.T) {
	// repo 404, owner is an org (200), creation succeeds (201).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repos/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/orgs/"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/orgs/"):
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	if err := ensureRepoExists("myorg/new-repo", "tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureRepoExists_CreateFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/repos/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/orgs/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"name already taken"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	patchHTTPClient(t, srv)
	withAPIBase(t, srv.URL)

	err := ensureRepoExists("alice/bad-repo", "tok")
	if err == nil {
		t.Fatal("expected an error when creation fails")
	}
}

func TestEnsureRepoExists_InvalidSlug(t *testing.T) {
	err := ensureRepoExists("not-a-slug", "tok")
	if err == nil {
		t.Fatal("expected an error for invalid slug")
	}
}
