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
