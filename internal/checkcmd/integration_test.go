package checkcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
)

func TestRunUsesRepoRootForProfileDetection(t *testing.T) {
	t.Parallel()

	repo := newGitRepoForCheck(t)
	writeRepoFile(t, filepath.Join(repo, "go.mod"), "module example.com/specbackfill-fixture\n\ngo 1.23\n")
	writeRepoFile(t, filepath.Join(repo, "openapi.yaml"), "openapi: 3.0.0\n")
	nested := filepath.Join(repo, "internal", "service")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	rootReport, _, rootStderr := runCheckJSON(t, repo, []string{"--format", "json"})
	nestedReport, _, nestedStderr := runCheckJSON(t, nested, []string{"--format", "json"})

	if rootStderr != "" || nestedStderr != "" {
		t.Fatalf("unexpected stderr root=%q nested=%q", rootStderr, nestedStderr)
	}
	if rootReport.RepoProfile != nestedReport.RepoProfile {
		t.Fatalf("repo profiles differ: root=%+v nested=%+v", rootReport.RepoProfile, nestedReport.RepoProfile)
	}
	if !rootReport.RepoProfile.Go || !rootReport.RepoProfile.OpenAPI {
		t.Fatalf("repo profile missing expected markers: %+v", rootReport.RepoProfile)
	}
}

func TestRunTextOutputUsesDiffLocalWording(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	stdout, stderr, code := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--fail-on", "off"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "[error] DB001 Schema changed, but no matching migration companion moved with this diff") {
		t.Fatalf("text output missing DB001 title:\n%s", stdout)
	}
	if !strings.Contains(stdout, "why: Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.") {
		t.Fatalf("text output missing why:\n%s", stdout)
	}
	if !strings.Contains(stdout, "evidence:") || !strings.Contains(stdout, "expected companions:") {
		t.Fatalf("text output missing evidence or companions block:\n%s", stdout)
	}
	if strings.Contains(strings.ToLower(stdout), "migration is missing") {
		t.Fatalf("text output violated diff-local wording:\n%s", stdout)
	}
	if strings.Contains(strings.ToLower(stdout), "no migration path moved") {
		t.Fatalf("text output over-claimed companion semantics:\n%s", stdout)
	}
}

func TestRunJSONOutputIncludesRequiredFindingFields(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	report, code, stderr := runCheckJSON(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "json", "--fail-on", "off"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("len(report.Findings) = %d, want 1", len(report.Findings))
	}

	finding := report.Findings[0]
	if finding.RuleID == "" || finding.Severity == "" || finding.Confidence == "" || finding.Title == "" || finding.Why == "" {
		t.Fatalf("finding has empty required scalar field: %+v", finding)
	}
	if len(finding.Evidence) == 0 {
		t.Fatalf("finding evidence missing: %+v", finding)
	}
	if len(finding.ExpectedCompanions) == 0 {
		t.Fatalf("expected companions missing: %+v", finding)
	}
}

func TestRunSummaryCountsAgreeBetweenTextAndJSON(t *testing.T) {
	t.Parallel()

	fixture := writeFixtureCopy(t, "db001_positive.diff")
	workdir := t.TempDir()

	textOutput, textStderr, textCode := runCheckText(t, workdir, []string{"--diff-file", fixture, "--format", "text", "--fail-on", "off"})
	if textCode != 0 || textStderr != "" {
		t.Fatalf("text run failed: code=%d stderr=%q", textCode, textStderr)
	}

	jsonReport, jsonCode, jsonStderr := runCheckJSON(t, workdir, []string{"--diff-file", fixture, "--format", "json", "--fail-on", "off"})
	if jsonCode != 0 || jsonStderr != "" {
		t.Fatalf("json run failed: code=%d stderr=%q", jsonCode, jsonStderr)
	}

	wantSummary := "findings: error=1 warn=0 info=0"
	if !strings.Contains(textOutput, wantSummary) {
		t.Fatalf("text output missing summary %q:\n%s", wantSummary, textOutput)
	}
	if jsonReport.Summary.Error != 1 || jsonReport.Summary.Warn != 0 || jsonReport.Summary.Info != 0 {
		t.Fatalf("json summary = %+v, want error=1 warn=0 info=0", jsonReport.Summary)
	}
}

func TestRunFailOnModesWithFindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fixture  string
		failOn   string
		wantCode int
	}{
		{name: "db001 error threshold", fixture: "db001_positive.diff", failOn: "error", wantCode: 1},
		{name: "db001 deleted migration still errors", fixture: "db001_deleted_migration.diff", failOn: "error", wantCode: 1},
		{name: "db001 warn threshold", fixture: "db001_positive.diff", failOn: "warn", wantCode: 1},
		{name: "db001 off threshold", fixture: "db001_positive.diff", failOn: "off", wantCode: 0},
		{name: "db002 error threshold", fixture: "db002_positive.diff", failOn: "error", wantCode: 0},
		{name: "db002 warn threshold", fixture: "db002_positive.diff", failOn: "warn", wantCode: 1},
		{name: "db002 removed-only companion clears warn threshold", fixture: "db002_removed_companion.diff", failOn: "warn", wantCode: 0},
		{name: "db002 paraphrased companion clears warn threshold", fixture: "db002_paraphrased_companion.diff", failOn: "warn", wantCode: 0},
		{name: "cfg001 error threshold", fixture: "cfg001_positive.diff", failOn: "error", wantCode: 0},
		{name: "cfg001 warn threshold", fixture: "cfg001_positive.diff", failOn: "warn", wantCode: 1},
		{name: "cfg001 removed unrelated docs still warn", fixture: "cfg001_removed_unrelated_docs.diff", failOn: "warn", wantCode: 1},
		{name: "cfg001 unrelated docs still warn", fixture: "cfg001_unrelated_docs.diff", failOn: "warn", wantCode: 1},
		{name: "api001 deleted docs still warn", fixture: "api001_deleted_docs.diff", failOn: "warn", wantCode: 1},
		{name: "err001 error threshold", fixture: "err001_positive.diff", failOn: "error", wantCode: 0},
		{name: "err001 warn threshold", fixture: "err001_positive.diff", failOn: "warn", wantCode: 1},
		{name: "err001 removed-only companion clears warn threshold", fixture: "err001_removed_companion.diff", failOn: "warn", wantCode: 0},
		{name: "err001 paraphrased companion clears warn threshold", fixture: "err001_paraphrased_companion.diff", failOn: "warn", wantCode: 0},
		{name: "ops001 error threshold", fixture: "ops001_positive.diff", failOn: "error", wantCode: 0},
		{name: "ops001 warn threshold", fixture: "ops001_positive.diff", failOn: "warn", wantCode: 1},
		{name: "ops001 companion clears warn threshold", fixture: "ops001_companion.diff", failOn: "warn", wantCode: 0},
		{name: "ops001 observability companion clears warn threshold", fixture: "ops001_observability_companion.diff", failOn: "warn", wantCode: 0},
		{name: "ops001 deleted companion still warns", fixture: "ops001_deleted_companion.diff", failOn: "warn", wantCode: 1},
		{name: "ops001 removed companion still warns", fixture: "ops001_removed_companion.diff", failOn: "warn", wantCode: 1},
		{name: "ops001 unrelated companion still warns", fixture: "ops001_unrelated_companion.diff", failOn: "warn", wantCode: 1},
		{name: "doc001 warn threshold", fixture: "doc001_positive.diff", failOn: "warn", wantCode: 1},
		{name: "doc001 unrelated docs still warn", fixture: "doc001_unrelated_docs.diff", failOn: "warn", wantCode: 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := writeFixtureCopy(t, tc.fixture)
			_, _, code := runCheckText(t, t.TempDir(), []string{"--diff-file", fixture, "--fail-on", tc.failOn})
			if code != tc.wantCode {
				t.Fatalf("Run() code = %d, want %d", code, tc.wantCode)
			}
		})
	}
}

func TestRunDiffFileCompanionHardening(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		want []string
	}{
		{name: "db001 deleted migration does not suppress", file: "db001_deleted_migration.diff", want: []string{"DB001"}},
		{name: "db001 unrelated migration does not suppress", file: "db001_unrelated_migration.diff", want: []string{"DB001"}},
		{name: "db001 true companion still suppresses", file: "db001_companion.diff", want: []string{}},
		{name: "db002 removed-only companion now suppresses", file: "db002_removed_companion.diff", want: []string{}},
		{name: "db002 paraphrased companion now suppresses", file: "db002_paraphrased_companion.diff", want: []string{}},
		{name: "db002 deleted companion does not suppress", file: "db002_deleted_companion.diff", want: []string{"DB002"}},
		{name: "db002 unrelated companion does not suppress", file: "db002_unrelated_companion.diff", want: []string{"DB002"}},
		{name: "db002 true companion still suppresses", file: "db002_companion.diff", want: []string{}},
		{name: "cfg001 deleted docs do not suppress", file: "cfg001_deleted_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 still emits when api001 is locally suppressed", file: "cfg001_positive_api001_suppressed.diff", want: []string{"CFG001"}},
		{name: "cfg001 removed unrelated docs do not suppress", file: "cfg001_removed_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 unrelated docs do not suppress", file: "cfg001_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 true companion still suppresses", file: "cfg001_companion.diff", want: []string{}},
		{name: "api001 deleted docs do not suppress", file: "api001_deleted_docs.diff", want: []string{"API001"}},
		{name: "api001 unrelated docs do not suppress", file: "api001_unrelated_docs.diff", want: []string{"API001"}},
		{name: "api001 true companion still suppresses", file: "api001_companion.diff", want: []string{}},
		{name: "err001 removed-only companion now suppresses", file: "err001_removed_companion.diff", want: []string{}},
		{name: "err001 paraphrased companion now suppresses", file: "err001_paraphrased_companion.diff", want: []string{}},
		{name: "err001 deleted companion does not suppress", file: "err001_deleted_companion.diff", want: []string{"ERR001"}},
		{name: "err001 unrelated companion does not suppress", file: "err001_unrelated_companion.diff", want: []string{"ERR001"}},
		{name: "err001 true companion still suppresses", file: "err001_companion.diff", want: []string{}},
		{name: "ops001 deleted companion does not suppress", file: "ops001_deleted_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 removed companion does not suppress", file: "ops001_removed_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 unrelated companion does not suppress", file: "ops001_unrelated_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 true companion still suppresses", file: "ops001_companion.diff", want: []string{}},
		{name: "ops001 observability companion still suppresses", file: "ops001_observability_companion.diff", want: []string{}},
		{name: "doc001 deleted docs do not suppress", file: "doc001_deleted_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 unrelated docs do not suppress", file: "doc001_unrelated_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 true companion still suppresses", file: "doc001_companion.diff", want: []string{}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := writeFixtureCopy(t, tc.file)
			report, code, stderr := runCheckJSON(t, t.TempDir(), []string{"--diff-file", fixture, "--format", "json", "--fail-on", "off"})
			if code != 0 || stderr != "" {
				t.Fatalf("Run() failed: code=%d stderr=%q", code, stderr)
			}
			if got := ruleIDsFromReport(report); !equalStrings(got, tc.want) {
				t.Fatalf("rule IDs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInputSourceEquivalenceByRule(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name     string
		wantRule string
		setup    func(t *testing.T, repo string)
		change   func(t *testing.T, repo string)
	}{
		{
			name:     "db001",
			wantRule: "DB001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n}\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n  email String @unique\n}\n")
			},
		},
		{
			name:     "db002",
			wantRule: "DB002",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "prisma", "migrations", "20260330010101_drop_user_email", "migration.sql"), "ALTER TABLE \"User\" ADD COLUMN \"email\" TEXT;\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "prisma", "migrations", "20260330010101_drop_user_email", "migration.sql"), "ALTER TABLE \"User\" ADD COLUMN \"email\" TEXT;\nALTER TABLE \"User\" DROP COLUMN \"email\";\n")
			},
		},
		{
			name:     "cfg001",
			wantRule: "CFG001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "config", "load.go"), "package config\n\nfunc load() {}\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "config", "load.go"), "package config\n\nfunc load() {\n  token := os.Getenv(\"NEW_API_TOKEN\")\n  _ = token\n}\n")
			},
		},
		{
			name:     "api001",
			wantRule: "API001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "openapi.yaml"), "paths:\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "openapi.yaml"), "paths:\n  /users:\n    get:\n      summary: list users\n")
			},
		},
		{
			name:     "err001",
			wantRule: "ERR001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "http", "handler.go"), "package http\n\nfunc statusCode() int {\n  return http.StatusBadRequest\n}\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "http", "handler.go"), "package http\n\nfunc statusCode() int {\n  return http.StatusUnprocessableEntity\n}\n")
			},
		},
		{
			name:     "doc001",
			wantRule: "DOC001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "generated", "openapi", "client.gen.ts"), "export type User = { id: string }\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "generated", "openapi", "client.gen.ts"), "export type User = { id: string }\nexport type UserList = User[]\n")
			},
		},
		{
			name:     "ops001",
			wantRule: "OPS001",
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "workers", "billing_consumer.go"), "package workers\n\nfunc NewBillingConsumer() Consumer {\n  return Consumer{Topic: \"billing.events\", RetryBackoff: 5 * time.Second}\n}\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "internal", "workers", "billing_consumer.go"), "package workers\n\nfunc NewBillingConsumer() Consumer {\n  return Consumer{Topic: \"billing.v2.events\", RetryBackoff: 30 * time.Second}\n}\n")
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			repo := newGitRepoForCheck(t)
			scenario.setup(t, repo)
			commitAll(t, repo, "base")
			base := strings.TrimSpace(runGitForCheck(t, repo, "rev-parse", "HEAD"))

			scenario.change(t, repo)
			workingReport, workingCode, workingStderr := runCheckJSON(t, repo, []string{"--format", "json", "--fail-on", "off"})
			if workingCode != 0 || workingStderr != "" {
				t.Fatalf("working tree run failed: code=%d stderr=%q", workingCode, workingStderr)
			}

			commitAll(t, repo, "head")
			head := strings.TrimSpace(runGitForCheck(t, repo, "rev-parse", "HEAD"))
			rangeReport, rangeCode, rangeStderr := runCheckJSON(t, repo, []string{"--base", base, "--head", head, "--format", "json", "--fail-on", "off"})
			if rangeCode != 0 || rangeStderr != "" {
				t.Fatalf("git range run failed: code=%d stderr=%q", rangeCode, rangeStderr)
			}

			patchPath := filepath.Join(t.TempDir(), scenario.name+".diff")
			writeRepoFile(t, patchPath, runGitForCheck(t, repo, "diff", base, head))
			diffReport, diffCode, diffStderr := runCheckJSON(t, repo, []string{"--diff-file", patchPath, "--format", "json", "--fail-on", "off"})
			if diffCode != 0 || diffStderr != "" {
				t.Fatalf("diff-file run failed: code=%d stderr=%q", diffCode, diffStderr)
			}

			want := []string{scenario.wantRule}
			if got := ruleIDsFromReport(workingReport); !equalStrings(got, want) {
				t.Fatalf("working tree rule IDs = %v, want %v", got, want)
			}
			if got := ruleIDsFromReport(rangeReport); !equalStrings(got, want) {
				t.Fatalf("git range rule IDs = %v, want %v", got, want)
			}
			if got := ruleIDsFromReport(diffReport); !equalStrings(got, want) {
				t.Fatalf("diff-file rule IDs = %v, want %v", got, want)
			}
		})
	}
}

func runCheckJSON(t *testing.T, cwd string, args []string) (model.Report, int, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), cwd, args, &stdout, &stderr)

	var report model.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}

	return report, code, stderr.String()
}

func runCheckText(t *testing.T, cwd string, args []string) (string, string, int) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), cwd, args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func newGitRepoForCheck(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGitForCheck(t, repo, "init")
	runGitForCheck(t, repo, "config", "user.name", "Spec Backfill")
	runGitForCheck(t, repo, "config", "user.email", "specbackfill@example.com")
	return repo
}

func runGitForCheck(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func commitAll(t *testing.T, repo, message string) {
	t.Helper()
	runGitForCheck(t, repo, "add", ".")
	runGitForCheck(t, repo, "commit", "-m", message)
}

func writeRepoFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeFixtureCopy(t *testing.T, fixtureName string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "patches", fixtureName))
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", fixtureName, err)
	}

	path := filepath.Join(t.TempDir(), fixtureName)
	writeRepoFile(t, path, string(data))
	return path
}

func ruleIDsFromReport(report model.Report) []string {
	ids := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		ids = append(ids, finding.RuleID)
	}
	return ids
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
