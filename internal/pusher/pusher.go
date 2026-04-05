// Package pusher clones the target GitHub repository and pushes generated Go
// code to the appropriate branch.
package pusher

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bangzzzz/pb-go-action/internal/config"
)

// Config holds the runtime parameters needed for the push operation.
type Config struct {
	// Token is the GitHub personal access token (or GITHUB_TOKEN) used to
	// authenticate HTTPS pushes.
	Token string

	// Branch is the target branch name to push to in the destination repo.
	Branch string

	// GitUserName is the commit author name.
	GitUserName string

	// GitUserEmail is the commit author e-mail.
	GitUserEmail string
}

// Push clones svc.TargetRepo, copies the contents of generatedDir into the
// cloned repository, and pushes to the configured branch.  If the branch does
// not exist in the remote, it is created from the repository's default branch.
func Push(svc *config.Service, generatedDir string, cfg Config) error {
	cloneDir, err := os.MkdirTemp("", "pb-go-action-push-*")
	if err != nil {
		return fmt.Errorf("creating clone dir: %w", err)
	}
	defer os.RemoveAll(cloneDir)

	repoURL := repoHTTPSURL(svc.TargetRepo, cfg.Token)

	// Create the target repository via the GitHub API if it does not exist yet.
	if err := ensureRepoExists(svc.TargetRepo, cfg.Token); err != nil {
		return fmt.Errorf("ensuring target repo exists: %w", err)
	}

	// Try to clone the specific branch; if it does not exist, clone the default
	// branch and create it locally.
	if err := cloneRepo(cloneDir, repoURL, cfg.Branch); err != nil {
		return err
	}

	// Configure git identity.
	if err := runGit(cloneDir, "config", "user.name", cfg.GitUserName); err != nil {
		return fmt.Errorf("setting git user.name: %w", err)
	}
	if err := runGit(cloneDir, "config", "user.email", cfg.GitUserEmail); err != nil {
		return fmt.Errorf("setting git user.email: %w", err)
	}

	// Copy generated files into the cloned repo, preserving directory structure.
	if err := copyDir(generatedDir, cloneDir); err != nil {
		return fmt.Errorf("copying generated files: %w", err)
	}

	// Optionally create/update go.mod.
	if svc.GoModule != "" {
		if err := ensureGoMod(cloneDir, svc.GoModule); err != nil {
			return fmt.Errorf("ensuring go.mod: %w", err)
		}
	}

	// Check whether anything actually changed.
	statusOut, err := runGitOutput(cloneDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(statusOut)) == "" {
		fmt.Printf("service %q: no changes to push\n", svc.Name)
		return nil
	}

	// Stage, commit and push.
	if err := runGit(cloneDir, "add", "--all"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	commitMsg := fmt.Sprintf("chore: update generated protobuf code [skip ci]")
	if err := runGit(cloneDir, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if err := runGit(cloneDir, "push", "origin", cfg.Branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	fmt.Printf("service %q: pushed generated code to %s (branch: %s)\n", svc.Name, svc.TargetRepo, cfg.Branch)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// cloneRepo clones repoURL into dir.  It first tries to check out branch; if
// that fails (branch not found in remote), it clones the default branch and
// creates branch locally.
func cloneRepo(dir, repoURL, branch string) error {
	err := runGitSilent(dir, "clone", "--depth=1", "--branch", branch, repoURL, ".")
	if err == nil {
		return nil
	}

	// Branch does not exist – clone default branch and create the new branch.
	if err2 := runGit(dir, "clone", "--depth=1", repoURL, "."); err2 != nil {
		return fmt.Errorf("cloning repo %s: %w", repoURL, err2)
	}
	if err2 := runGit(dir, "checkout", "-b", branch); err2 != nil {
		return fmt.Errorf("creating branch %q: %w", branch, err2)
	}
	return nil
}

// repoHTTPSURL builds an authenticated HTTPS URL for a GitHub repo slug.
func repoHTTPSURL(slug, token string) string {
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, slug)
}

// runGit runs a git command in dir, streaming stdout/stderr to os.Stdout/Stderr.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGitSilent runs a git command suppressing output.  Useful when we expect
// the command might fail (e.g. cloning a non-existent branch).
func runGitSilent(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// runGitOutput runs a git command and captures its stdout.
func runGitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return buf.Bytes(), err
}

// copyDir copies all files from src into dst, recreating the directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// ensureGoMod creates a minimal go.mod in dir if one does not already exist.
func ensureGoMod(dir, moduleName string) error {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		// go.mod already exists; leave it as-is.
		return nil
	}
	content := fmt.Sprintf("module %s\n\ngo 1.22\n", moduleName)
	return os.WriteFile(goModPath, []byte(content), 0o644)
}

// ── GitHub repository helpers ─────────────────────────────────────────────────

// apiBase is the GitHub API root URL.  It is a variable so tests can override
// it to point at a local httptest server.
var apiBase = "https://api.github.com"

// ensureRepoExists creates the GitHub repository identified by slug
// ("owner/repo") via the GitHub API if it does not already exist.
func ensureRepoExists(slug, token string) error {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo slug %q (expected owner/repo)", slug)
	}
	owner, repo := parts[0], parts[1]

	exists, err := repoExists(owner, repo, token)
	if err != nil {
		return fmt.Errorf("checking whether repo %s exists: %w", slug, err)
	}
	if exists {
		return nil
	}

	fmt.Printf("target repository %s not found – creating via GitHub API …\n", slug)

	// Choose the correct creation endpoint depending on whether the owner is a
	// GitHub organisation or a plain user account.
	isOrg, err := ownerIsOrg(owner, token)
	if err != nil {
		return fmt.Errorf("determining owner type for %q: %w", owner, err)
	}

	var apiURL string
	if isOrg {
		apiURL = fmt.Sprintf("%s/orgs/%s/repos", apiBase, owner)
	} else {
		apiURL = fmt.Sprintf("%s/user/repos", apiBase)
	}

	body := fmt.Sprintf(`{"name":%q,"private":false,"auto_init":true}`, repo)
	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("building create-repo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("creating repo %s: %w", slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("creating repo %s: GitHub API returned %d: %s", slug, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	fmt.Printf("created repository %s\n", slug)
	return nil
}

// repoExists reports whether the GitHub repository owner/repo is accessible
// with the given token.
func repoExists(owner, repo, token string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", apiBase, owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status %d checking repo %s/%s", resp.StatusCode, owner, repo)
	}
}

// ownerIsOrg reports whether the GitHub account named owner is an organisation.
func ownerIsOrg(owner, token string) (bool, error) {
	url := fmt.Sprintf("%s/orgs/%s", apiBase, owner)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK, nil
}
