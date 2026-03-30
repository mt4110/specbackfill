package diffparse

import (
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
)

func TestParseGitDiffHeaderAndPatch(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git a/file.txt b/file.txt\nindex 1111111..2222222 100644\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}
	if diff.Files[0].Path != "file.txt" {
		t.Fatalf("file.Path = %q, want %q", diff.Files[0].Path, "file.txt")
	}
}

func TestParseRenameHeadersPreserveLeadingAAndBPaths(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git a/a/file.txt b/b/file.txt\nsimilarity index 100%\nrename from a/file.txt\nrename to b/file.txt\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}
	if diff.Files[0].Status != model.FileStatusRenamed {
		t.Fatalf("file.Status = %q, want %q", diff.Files[0].Status, model.FileStatusRenamed)
	}
	if diff.Files[0].OldPath != "a/file.txt" {
		t.Fatalf("file.OldPath = %q, want %q", diff.Files[0].OldPath, "a/file.txt")
	}
	if diff.Files[0].NewPath != "b/file.txt" {
		t.Fatalf("file.NewPath = %q, want %q", diff.Files[0].NewPath, "b/file.txt")
	}
}

func TestParseCopyHeadersPreserveLeadingAAndBPaths(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git a/a/file.txt b/b/file.txt\nsimilarity index 100%\ncopy from a/file.txt\ncopy to b/file.txt\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}
	if diff.Files[0].Status != model.FileStatusCopied {
		t.Fatalf("file.Status = %q, want %q", diff.Files[0].Status, model.FileStatusCopied)
	}
	if diff.Files[0].OldPath != "a/file.txt" {
		t.Fatalf("file.OldPath = %q, want %q", diff.Files[0].OldPath, "a/file.txt")
	}
	if diff.Files[0].NewPath != "b/file.txt" {
		t.Fatalf("file.NewPath = %q, want %q", diff.Files[0].NewPath, "b/file.txt")
	}
}
