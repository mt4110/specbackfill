package fixturecmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunReport(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), "../..", []string{"report"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run(report) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	for _, want := range []string{
		"Rule    Positive  Companion  Negative  Edge",
		"DB001",
		"DOC001",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("fixture report missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing subcommand", args: nil, want: "usage: specbackfill fixtures report"},
		{name: "unknown subcommand", args: []string{"nope"}, want: `unknown fixtures command "nope"`},
		{name: "report extra arg", args: []string{"report", "extra"}, want: "fixtures report does not accept positional arguments"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(context.Background(), "../..", tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("Run(%v) code = %d, want 2", tc.args, code)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr.String())
			}
		})
	}
}
