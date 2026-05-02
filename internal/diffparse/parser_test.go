package diffparse

import (
	"errors"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
)

func TestParseNormalizesModel(t *testing.T) {
	t.Parallel()

	input := []byte("--- a\\dir\\file.txt\n+++ b\\dir\\file.txt\n@@ -1 +1 @@\n-old value\n+new value\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}

	file := diff.Files[0]
	if file.Path != "dir/file.txt" {
		t.Fatalf("file.Path = %q, want %q", file.Path, "dir/file.txt")
	}
	if file.Status != model.FileStatusModified {
		t.Fatalf("file.Status = %q, want %q", file.Status, model.FileStatusModified)
	}
	if len(file.Hunks) != 1 {
		t.Fatalf("len(file.Hunks) = %d, want 1", len(file.Hunks))
	}
	if len(file.Hunks[0].Lines) != 2 {
		t.Fatalf("len(file.Hunks[0].Lines) = %d, want 2", len(file.Hunks[0].Lines))
	}
	if file.Hunks[0].Lines[0].Kind != model.LineKindRemoved {
		t.Fatalf("first line kind = %q, want %q", file.Hunks[0].Lines[0].Kind, model.LineKindRemoved)
	}
	if file.Hunks[0].Lines[1].Kind != model.LineKindAdded {
		t.Fatalf("second line kind = %q, want %q", file.Hunks[0].Lines[1].Kind, model.LineKindAdded)
	}
}

func TestParseMalformedDiff(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte("@@ -1 +1 @@\n+broken\n"))
	if !errors.Is(err, ErrMalformedDiff) {
		t.Fatalf("Parse() error = %v, want ErrMalformedDiff", err)
	}
}

func TestParseTruncatedHunk(t *testing.T) {
	t.Parallel()

	input := []byte("--- a/file.txt\n+++ b/file.txt\n@@ -1,2 +1,2 @@\n old\n")

	_, err := Parse(input)
	if !errors.Is(err, ErrMalformedDiff) {
		t.Fatalf("Parse() error = %v, want ErrMalformedDiff", err)
	}
}

func TestParseMixedNewlines(t *testing.T) {
	t.Parallel()

	input := []byte("--- a/schema.prisma\r\n+++ b/schema.prisma\n@@ -1,3 +1,4 @@\r\n model User {\n   id Int @id\r\n+  email String @unique\r\n }\n")

	diff, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(diff.Files) != 1 {
		t.Fatalf("len(diff.Files) = %d, want 1", len(diff.Files))
	}
	if len(diff.Files[0].Hunks) != 1 {
		t.Fatalf("len(diff.Files[0].Hunks) = %d, want 1", len(diff.Files[0].Hunks))
	}

	lines := diff.Files[0].Hunks[0].Lines
	if len(lines) != 4 {
		t.Fatalf("len(lines) = %d, want 4", len(lines))
	}
	if lines[2].Text != "  email String @unique" {
		t.Fatalf("lines[2].Text = %q, want %q", lines[2].Text, "  email String @unique")
	}
}
