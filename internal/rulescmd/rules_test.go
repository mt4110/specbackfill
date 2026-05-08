package rulescmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/rules"
)

func TestRunList(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRules(t, []string{"list"})
	if code != 0 {
		t.Fatalf("Run(list) code = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.HasPrefix(stdout, "Rule    Severity  Description\n") {
		t.Fatalf("list output missing header:\n%s", stdout)
	}

	for _, info := range rules.Catalog() {
		if count := strings.Count(stdout, info.ID); count != 1 {
			t.Fatalf("list output contains %s %d times, want 1:\n%s", info.ID, count, stdout)
		}
		if !strings.Contains(stdout, string(info.DefaultSeverity)) {
			t.Fatalf("list output missing severity %q:\n%s", info.DefaultSeverity, stdout)
		}
		if !strings.Contains(stdout, info.ListDescription) {
			t.Fatalf("list output missing description %q:\n%s", info.ListDescription, stdout)
		}
	}
}

func TestRunShowIncludesRequiredSectionsForEveryRule(t *testing.T) {
	t.Parallel()

	for _, info := range rules.Catalog() {
		info := info
		t.Run(info.ID, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := runRules(t, []string{"show", info.ID})
			if code != 0 {
				t.Fatalf("Run(show %s) code = %d, want 0; stderr=%q", info.ID, code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}

			required := []string{
				info.ID + ": " + info.Title,
				"Default severity: " + string(info.DefaultSeverity),
				"What triggers it:",
				"Expected companions:",
				"Does not report on:",
			}
			for _, want := range required {
				if !strings.Contains(stdout, want) {
					t.Fatalf("show output missing %q:\n%s", want, stdout)
				}
			}
			for _, companion := range info.ExpectedCompanions {
				if !strings.Contains(stdout, "  - "+companion) {
					t.Fatalf("show output missing expected companion %q:\n%s", companion, stdout)
				}
			}
		})
	}
}

func TestRunShowAcceptsLowercaseRuleID(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRules(t, []string{"show", "db001"})
	if code != 0 {
		t.Fatalf("Run(show db001) code = %d, want 0; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "DB001:") {
		t.Fatalf("show output did not canonicalize rule ID:\n%s", stdout)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing subcommand", args: nil, want: "usage: specbackfill rules list"},
		{name: "unknown subcommand", args: []string{"nope"}, want: `unknown rules command "nope"`},
		{name: "list extra arg", args: []string{"list", "extra"}, want: "rules list does not accept positional arguments"},
		{name: "show missing id", args: []string{"show"}, want: "usage: specbackfill rules show <RULE_ID>"},
		{name: "show extra arg", args: []string{"show", "DB001", "extra"}, want: "usage: specbackfill rules show <RULE_ID>"},
		{name: "unknown rule id", args: []string{"show", "NOPE001"}, want: `unknown rule ID "NOPE001"`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, code := runRules(t, tc.args)
			if code != 2 {
				t.Fatalf("Run(%v) code = %d, want 2", tc.args, code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr)
			}
		})
	}
}

func runRules(t *testing.T, args []string) (string, string, int) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}
