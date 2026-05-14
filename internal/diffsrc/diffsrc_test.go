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

func TestWorkingTreeIncludesStagedChanges(t *testing.T) {
	t.Parallel()

	repo := newGitRepo(t)
	writeFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n}\n")
	runGit(t, repo, "add", "schema.prisma")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n  email String @unique\n}\n")
	runGit(t, repo, "add", "schema.prisma")

	diff, err := WorkingTree(context.Background(), repo)
	if err != nil {
		t.Fatalf("WorkingTree() error = %v", err)
	}
	if !strings.Contains(string(diff), "email String @unique") {
		t.Fatalf("WorkingTree() diff does not include staged schema change:\n%s", diff)
	}
}

func TestWorkingTreeIncludesStagedChangesWithoutHead(t *testing.T) {
	t.Parallel()

	repo := newGitRepo(t)
	writeFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n  email String @unique\n}\n")
	runGit(t, repo, "add", "schema.prisma")

	diff, err := WorkingTree(context.Background(), repo)
	if err != nil {
		t.Fatalf("WorkingTree() error = %v", err)
	}
	if !strings.Contains(string(diff), "email String @unique") {
		t.Fatalf("WorkingTree() diff does not include unborn-branch staged schema change:\n%s", diff)
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

	got, err := DiffFile(t.TempDir(), path)
	if err != nil {
		t.Fatalf("DiffFile() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("DiffFile() = %q, want %q", got, want)
	}
}

func TestDiffFileResolvesRelativePathFromDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	writeFile(t, filepath.Join(dir, "sample.diff"), want)

	got, err := DiffFile(dir, "sample.diff")
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

func TestRepoRootFallsBackToFileLayoutWithoutGit(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "go.mod"), "module github.com/mt4110/specbackfill\n")
	if err := os.MkdirAll(filepath.Join(repo, "cmd", "specbackfill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(cmd/specbackfill) error = %v", err)
	}
	writeFile(t, filepath.Join(repo, "cmd", "specbackfill", "main.go"), "package main\n")
	if err := os.MkdirAll(filepath.Join(repo, "schemas"), 0o755); err != nil {
		t.Fatalf("MkdirAll(schemas) error = %v", err)
	}
	writeFile(t, filepath.Join(repo, "schemas", "obligations.schema.json"), "{}\n")
	if err := os.MkdirAll(filepath.Join(repo, "testdata", "patches"), 0o755); err != nil {
		t.Fatalf("MkdirAll(testdata/patches) error = %v", err)
	}
	nested := filepath.Join(repo, "cmd", "specbackfill")

	root, err := RepoRoot(context.Background(), nested)
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	if root != repo {
		t.Fatalf("RepoRoot() = %q, want %q", root, repo)
	}
}

func TestRepoRootReportsGitAndFallbackFailures(t *testing.T) {
	t.Parallel()

	_, err := RepoRoot(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("RepoRoot() error = nil, want error")
	}
	for _, want := range []string{
		"git rev-parse failed",
		"file-layout fallback failed",
		"no specbackfill file-layout root found",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("RepoRoot() error missing %q:\n%v", want, err)
		}
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
