package diffsrc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingTree(t *testing.T) {
	t.Parallel()

	repo := newGitRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "before\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, filepath.Join(repo, "tracked.txt"), "after\n")

	diff, err := WorkingTree(context.Background(), repo)
	if err != nil {
		t.Fatalf("WorkingTree() error = %v", err)
	}
	if !strings.Contains(string(diff), "tracked.txt") {
		t.Fatalf("WorkingTree() diff does not mention tracked.txt:\n%s", diff)
	}
}

func TestGitRange(t *testing.T) {
	t.Parallel()

	repo := newGitRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "before\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "initial")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	writeFile(t, filepath.Join(repo, "tracked.txt"), "after\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "update")
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	diff, err := GitRange(context.Background(), repo, base, head)
	if err != nil {
		t.Fatalf("GitRange() error = %v", err)
	}
	if !strings.Contains(string(diff), "tracked.txt") {
		t.Fatalf("GitRange() diff does not mention tracked.txt:\n%s", diff)
	}
}

func TestDiffFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.diff")
	want := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	writeFile(t, path, want)

	got, err := DiffFile(path)
	if err != nil {
		t.Fatalf("DiffFile() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("DiffFile() = %q, want %q", got, want)
	}
}

func TestRepoRootFromNestedDirectory(t *testing.T) {
	t.Parallel()

	repo := newGitRepo(t)
	nested := filepath.Join(repo, "internal", "service")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	root, err := RepoRoot(context.Background(), nested)
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks(root) error = %v", err)
	}
	repoResolved, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("EvalSymlinks(repo) error = %v", err)
	}
	if rootResolved != repoResolved {
		t.Fatalf("RepoRoot() = %q, want %q", rootResolved, repoResolved)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Spec Backfill")
	runGit(t, repo, "config", "user.email", "specbackfill@example.com")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
