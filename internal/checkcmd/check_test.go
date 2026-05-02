package checkcmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInvalidFlagCombinations(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"--base", "HEAD"},
		{"--head", "HEAD"},
		{"--base", "HEAD~1", "--head", "HEAD", "--diff-file", "sample.diff"},
	}

	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(context.Background(), t.TempDir(), args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("Run() code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Fatalf("stderr is empty")
			}
		})
	}
}

func TestRunHelpExitsZero(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), t.TempDir(), []string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage of check:") {
		t.Fatalf("stderr = %q, want usage output", stderr.String())
	}
}

func TestRunMalformedDiffFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	diffPath := filepath.Join(dir, "broken.diff")
	if err := os.WriteFile(diffPath, []byte("not a diff\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), dir, []string{"--diff-file", diffPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "malformed unified diff") {
		t.Fatalf("stderr = %q, want malformed diff error", stderr.String())
	}
}

func TestRunDiffFileJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	diffPath := filepath.Join(dir, "change.diff")
	diffText := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if err := os.WriteFile(diffPath, []byte(diffText), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), dir, []string{"--diff-file", diffPath, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "v0"`) {
		t.Fatalf("stdout = %q, want JSON skeleton", stdout.String())
	}
}

func TestRunDiffFileUsesProvidedCwdForRelativePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	diffPath := filepath.Join(dir, "change.diff")
	diffText := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if err := os.WriteFile(diffPath, []byte(diffText), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), dir, []string{"--diff-file", "change.diff", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version": "v0"`) {
		t.Fatalf("stdout = %q, want JSON skeleton", stdout.String())
	}
}

func TestRunFailOnModesWithoutRules(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	diffPath := filepath.Join(dir, "change.diff")
	diffText := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if err := os.WriteFile(diffPath, []byte(diffText), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	for _, failOn := range []string{"error", "warn", "off"} {
		failOn := failOn
		t.Run(failOn, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(context.Background(), dir, []string{"--diff-file", diffPath, "--fail-on", failOn}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr.String())
			}
		})
	}
}
