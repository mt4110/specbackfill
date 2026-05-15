package diffinput

import (
	"strings"
	"testing"
)

func TestSelectionMetadata(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		selection   Selection
		wantKind    string
		wantSummary string
		wantNote    string
	}{
		{
			name:        "working tree",
			wantKind:    "working_tree",
			wantSummary: "working tree diff (tracked changes)",
			wantNote:    "untracked files are not included unless staged with git add -N",
		},
		{
			name:        "range",
			selection:   Selection{Base: "main", Head: "HEAD"},
			wantKind:    "range",
			wantSummary: "git range diff (main..HEAD)",
			wantNote:    "working tree changes are not included in --base/--head mode",
		},
		{
			name:        "diff file",
			selection:   Selection{DiffFile: "change.diff"},
			wantKind:    "diff_file",
			wantSummary: "diff file",
			wantNote:    "only the provided unified diff file was evaluated",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := Kind(tc.selection); got != tc.wantKind {
				t.Fatalf("Kind() = %q, want %q", got, tc.wantKind)
			}
			if got := Summary(tc.selection); got != tc.wantSummary {
				t.Fatalf("Summary() = %q, want %q", got, tc.wantSummary)
			}
			notes := Notes(tc.selection)
			if len(notes) != 1 || notes[0] != tc.wantNote {
				t.Fatalf("Notes() = %v, want [%q]", notes, tc.wantNote)
			}
		})
	}
}

func TestValidateSelection(t *testing.T) {
	t.Parallel()

	valid := []Selection{
		{},
		{Base: "main", Head: "HEAD"},
		{DiffFile: "change.diff"},
	}
	for _, selection := range valid {
		if err := Validate(selection); err != nil {
			t.Fatalf("Validate(%+v) error = %v, want nil", selection, err)
		}
	}

	invalid := []struct {
		selection Selection
		want      string
	}{
		{
			selection: Selection{Base: "main"},
			want:      "--base and --head must be provided together",
		},
		{
			selection: Selection{Head: "HEAD"},
			want:      "--base and --head must be provided together",
		},
		{
			selection: Selection{Base: "main", Head: "HEAD", DiffFile: "change.diff"},
			want:      "--diff-file cannot be combined with --base/--head",
		},
	}
	for _, tc := range invalid {
		err := Validate(tc.selection)
		if err == nil {
			t.Fatalf("Validate(%+v) error = nil, want %q", tc.selection, tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Validate(%+v) error = %q, want %q", tc.selection, err.Error(), tc.want)
		}
	}
}
