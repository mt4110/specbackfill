package checkcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
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

func TestInputSummaryAndNotes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		base        string
		head        string
		diffFile    string
		wantSummary string
		wantNote    string
	}{
		{
			name:        "working tree",
			wantSummary: "working tree diff (tracked changes)",
			wantNote:    "untracked files are not included unless staged with git add -N",
		},
		{
			name:        "git range",
			base:        "main",
			head:        "HEAD",
			wantSummary: "git range diff (main..HEAD)",
			wantNote:    "working tree changes are not included in --base/--head mode",
		},
		{
			name:        "diff file",
			diffFile:    "change.diff",
			wantSummary: "diff file",
			wantNote:    "only the provided unified diff file was evaluated",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := inputSummary(tc.base, tc.head, tc.diffFile); got != tc.wantSummary {
				t.Fatalf("inputSummary() = %q, want %q", got, tc.wantSummary)
			}
			notes := inputNotes(tc.base, tc.head, tc.diffFile)
			if len(notes) != 1 || notes[0] != tc.wantNote {
				t.Fatalf("inputNotes() = %v, want [%q]", notes, tc.wantNote)
			}
		})
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

func TestRunEmitObligationsJSON(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	for _, args := range [][]string{
		{"--diff-file", fixture, "--emit-obligations", "--fail-on", "off"},
		{"--diff-file", fixture, "--emit-obligations", "--format", "json", "--fail-on", "off"},
	} {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()

			artifact, code, stderr := runCheckObligations(t, t.TempDir(), args)
			if code != 0 {
				t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if artifact.SchemaVersion != "obligations.v1" {
				t.Fatalf("SchemaVersion = %q, want obligations.v1", artifact.SchemaVersion)
			}
			if len(artifact.Obligations) != 1 {
				t.Fatalf("len(obligations) = %d, want 1", len(artifact.Obligations))
			}
			obligation := artifact.Obligations[0]
			if obligation.RuleID != "DB001" || obligation.Status != "missing" {
				t.Fatalf("obligation = %+v, want DB001 missing", obligation)
			}
			if obligation.FindingID == nil {
				t.Fatalf("missing obligation did not link to finding ID: %+v", obligation)
			}
			if obligation.Anchor.Path == "" || len(obligation.Anchor.Evidence) == 0 {
				t.Fatalf("missing obligation lacks anchor evidence: %+v", obligation)
			}
		})
	}
}

func TestRunEmitObligationsCannotCombineWithSummary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "summary",
			args: []string{"--emit-obligations", "--summary"},
			want: "--emit-obligations cannot be combined with --summary",
		},
		{
			name: "explain",
			args: []string{"--emit-obligations", "--explain"},
			want: "--emit-obligations cannot be combined with --explain",
		},
		{
			name: "text format",
			args: []string{"--emit-obligations", "--format", "text"},
			want: "--emit-obligations can only be combined with --format json",
		},
		{
			name: "markdown format",
			args: []string{"--emit-obligations", "--format", "markdown"},
			want: "--emit-obligations can only be combined with --format json",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			code := Run(context.Background(), t.TempDir(), tc.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("Run() code = %d, want 2", code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}

func TestRunDiffFileMarkdown(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	stdout, stderr, code := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "markdown", "--fail-on", "off"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "### specbackfill findings") {
		t.Fatalf("markdown output missing heading:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#### [error] DB001") {
		t.Fatalf("markdown output missing DB001 finding:\n%s", stdout)
	}
	if !strings.Contains(stdout, "`schema.prisma`: `+ email String @unique`") {
		t.Fatalf("markdown output missing evidence:\n%s", stdout)
	}
}

func TestRunDiffFileSummary(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "api001_err001_positive.diff")
	stdout, stderr, code := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "text", "--summary", "--fail-on", "off"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"specbackfill summary",
		"warn:  2",
		"- API001: 1",
		"- ERR001: 1",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("summary output missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "expected companions:") {
		t.Fatalf("summary output included finding details:\n%s", stdout)
	}
}

func TestRunDiffFileSummaryWithExplainMatchesSummary(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "api001_err001_positive.diff")
	summary, summaryStderr, summaryCode := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "text", "--summary", "--fail-on", "off"})
	explainedSummary, explainedSummaryStderr, explainedSummaryCode := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "text", "--summary", "--explain", "--fail-on", "off"})
	if summaryCode != 0 || explainedSummaryCode != 0 {
		t.Fatalf("Run() codes summary=%d explained=%d; stderr summary=%q explained=%q", summaryCode, explainedSummaryCode, summaryStderr, explainedSummaryStderr)
	}
	if summaryStderr != "" || explainedSummaryStderr != "" {
		t.Fatalf("unexpected stderr summary=%q explained=%q", summaryStderr, explainedSummaryStderr)
	}
	if explainedSummary != summary {
		t.Fatalf("summary output changed with --explain\nsummary:\n%s\nexplained:\n%s", summary, explainedSummary)
	}
}

func TestRunDiffFileJSONWithExplain(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	report, code, stderr := runCheckJSON(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "json", "--fail-on", "off", "--explain"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("len(report.Findings) = %d, want 1", len(report.Findings))
	}
	if len(report.Explanations) != 1 {
		t.Fatalf("len(report.Explanations) = %d, want 1", len(report.Explanations))
	}

	finding := report.Findings[0]
	explanation := report.Explanations[0]
	if explanation.RuleID != finding.RuleID {
		t.Fatalf("explanation.RuleID = %q, want %q", explanation.RuleID, finding.RuleID)
	}
	if explanation.FindingIndex != 0 {
		t.Fatalf("explanation.FindingIndex = %d, want 0", explanation.FindingIndex)
	}
	if len(explanation.Evidence) != len(finding.Evidence) {
		t.Fatalf("explanation evidence = %+v, want %+v", explanation.Evidence, finding.Evidence)
	}
	if len(explanation.ExpectedCompanions) != len(finding.ExpectedCompanions) {
		t.Fatalf("explanation companions = %+v, want %+v", explanation.ExpectedCompanions, finding.ExpectedCompanions)
	}
	if !strings.Contains(explanation.Summary, "does not claim repository-wide absence") {
		t.Fatalf("explanation summary = %q, want diff-local guardrail", explanation.Summary)
	}
}

func TestRunDiffFileExplainPreservesFindingsAndExitCode(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")

	baseReport, baseCode, baseStderr := runCheckJSON(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "json", "--fail-on", "error"})
	explainedReport, explainedCode, explainedStderr := runCheckJSON(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "json", "--fail-on", "error", "--explain"})
	if baseStderr != "" || explainedStderr != "" {
		t.Fatalf("unexpected stderr base=%q explained=%q", baseStderr, explainedStderr)
	}
	if baseCode != explainedCode {
		t.Fatalf("exit codes differ: base=%d explained=%d", baseCode, explainedCode)
	}
	if baseCode != 1 {
		t.Fatalf("Run() code = %d, want 1", baseCode)
	}
	if baseReport.Summary != explainedReport.Summary {
		t.Fatalf("summaries differ: base=%+v explained=%+v", baseReport.Summary, explainedReport.Summary)
	}
	if !equalStrings(ruleIDsFromReport(baseReport), ruleIDsFromReport(explainedReport)) {
		t.Fatalf("rule IDs differ: base=%v explained=%v", ruleIDsFromReport(baseReport), ruleIDsFromReport(explainedReport))
	}
	if len(baseReport.Explanations) != 0 {
		t.Fatalf("base explanations = %+v, want empty", baseReport.Explanations)
	}
	if len(explainedReport.Explanations) != len(explainedReport.Findings) {
		t.Fatalf("explained report has %d explanations for %d findings", len(explainedReport.Explanations), len(explainedReport.Findings))
	}
}

func TestRunDiffFileTextWithExplain(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	stdout, stderr, code := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "text", "--fail-on", "off", "--explain"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "  explanation: This explains the existing DB001 finding") {
		t.Fatalf("text output missing explanation:\n%s", stdout)
	}
	if !strings.Contains(stdout, "schema.prisma:3:+ email String @unique") {
		t.Fatalf("text output missing evidence reference:\n%s", stdout)
	}
	if strings.Contains(strings.ToLower(stdout), "migration is missing") {
		t.Fatalf("text output violated diff-local wording:\n%s", stdout)
	}
}

func TestRunDiffFileExplainWithoutFindingsKeepsBaseOutputAvailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	diffPath := filepath.Join(dir, "change.diff")
	diffText := "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-old\n+new\n"
	if err := os.WriteFile(diffPath, []byte(diffText), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, code, stderr := runCheckJSON(t, dir, []string{"--diff-file", diffPath, "--format", "json", "--fail-on", "off", "--explain"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("len(report.Findings) = %d, want 0", len(report.Findings))
	}
	if len(report.Explanations) != 0 {
		t.Fatalf("len(report.Explanations) = %d, want 0", len(report.Explanations))
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

func runCheckObligations(t *testing.T, cwd string, args []string) (model.ObligationArtifact, int, string) {
	t.Helper()

	stdout, code, stderr := runCheckOutput(t, cwd, args)
	var artifact model.ObligationArtifact
	if err := json.Unmarshal([]byte(stdout), &artifact); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw=%s", err, stdout)
	}
	return artifact, code, stderr
}
