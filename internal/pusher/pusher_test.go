package pusher

import (
	"os"
	"path/filepath"
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
