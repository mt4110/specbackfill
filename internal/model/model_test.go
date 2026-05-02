package model

import "testing"

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	got := NormalizePath(`dir\subdir\..\file.txt`)
	want := "dir/file.txt"
	if got != want {
		t.Fatalf("NormalizePath() = %q, want %q", got, want)
	}
}
