package diffparse

import "testing"

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
