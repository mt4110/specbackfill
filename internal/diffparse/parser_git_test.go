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

func TestParseQuotedPathsWithSpaces(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git \"a/dir with space/file name.txt\" \"b/dir with space/file name.txt\"\nindex 1111111..2222222 100644\n--- \"a/dir with space/file name.txt\"\n+++ \"b/dir with space/file name.txt\"\n@@ -1 +1 @@\n-old\n+new\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}

	file := diff.Files[0]
	if file.Path != "dir with space/file name.txt" {
		t.Fatalf("file.Path = %q, want %q", file.Path, "dir with space/file name.txt")
	}
	if file.OldPath != "dir with space/file name.txt" {
		t.Fatalf("file.OldPath = %q, want %q", file.OldPath, "dir with space/file name.txt")
	}
	if file.NewPath != "dir with space/file name.txt" {
		t.Fatalf("file.NewPath = %q, want %q", file.NewPath, "dir with space/file name.txt")
	}
}

func TestParseQuotedRenameAndCopyMetadataPathsWithSpaces(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git \"a/a/source name.txt\" \"b/b/target name.txt\"\nsimilarity index 100%\ncopy from \"a/source name.txt\"\ncopy to \"b/target name.txt\"\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}

	file := diff.Files[0]
	if file.Status != model.FileStatusCopied {
		t.Fatalf("file.Status = %q, want %q", file.Status, model.FileStatusCopied)
	}
	if file.OldPath != "a/source name.txt" {
		t.Fatalf("file.OldPath = %q, want %q", file.OldPath, "a/source name.txt")
	}
	if file.NewPath != "b/target name.txt" {
		t.Fatalf("file.NewPath = %q, want %q", file.NewPath, "b/target name.txt")
	}
}

func TestParseMetadataOnlyFileDoesNotCorruptNextFile(t *testing.T) {
	t.Parallel()

	input := []byte("diff --git \"a/docs/old guide.md\" \"b/docs/new guide.md\"\nsimilarity index 100%\nrename from \"docs/old guide.md\"\nrename to \"docs/new guide.md\"\ndiff --git a/schema.prisma b/schema.prisma\nindex 1111111..2222222 100644\n--- a/schema.prisma\n+++ b/schema.prisma\n@@ -1,3 +1,4 @@\n model User {\n   id Int @id\n+  email String @unique\n }\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 2 {
		t.Fatalf("len(diff.Files) = %d, want 2", len(diff.Files))
	}

	first := diff.Files[0]
	if first.Status != model.FileStatusRenamed {
		t.Fatalf("first.Status = %q, want %q", first.Status, model.FileStatusRenamed)
	}
	if first.Path != "docs/new guide.md" {
		t.Fatalf("first.Path = %q, want %q", first.Path, "docs/new guide.md")
	}
	if len(first.Hunks) != 0 {
		t.Fatalf("len(first.Hunks) = %d, want 0", len(first.Hunks))
	}

	second := diff.Files[1]
	if second.Path != "schema.prisma" {
		t.Fatalf("second.Path = %q, want %q", second.Path, "schema.prisma")
	}
	if len(second.Hunks) != 1 {
		t.Fatalf("len(second.Hunks) = %d, want 1", len(second.Hunks))
	}
	if second.Hunks[0].Lines[2].Text != "  email String @unique" {
		t.Fatalf("second added line = %q, want %q", second.Hunks[0].Lines[2].Text, "  email String @unique")
	}
}
