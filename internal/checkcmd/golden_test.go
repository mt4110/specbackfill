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

func TestGoldenOutputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fixture string
	}{
		{name: "db001_positive", fixture: "db001_positive.diff"},
		{name: "db002_positive", fixture: "db002_positive.diff"},
		{name: "cfg001_positive", fixture: "cfg001_positive.diff"},
		{name: "api001_positive", fixture: "api001_positive.diff"},
		{name: "err001_positive", fixture: "err001_positive.diff"},
		{name: "doc001_positive", fixture: "doc001_positive.diff"},
		{name: "db001_companion", fixture: "db001_companion.diff"},
		{name: "db001_db002_positive", fixture: "db001_db002_positive.diff"},
		{name: "api001_err001_positive", fixture: "api001_err001_positive.diff"},
		{name: "db001_migration_only", fixture: "db001_migration_only.diff"},
	}

	for _, tc := range cases {
		tc := tc
		for _, format := range []string{"text", "json"} {
			format := format
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				t.Parallel()

				fixture := writeFixtureCopy(t, tc.fixture)
				stdout, code, stderr := runCheckOutput(t, t.TempDir(), []string{"--diff-file", fixture, "--format", format, "--fail-on", "off"})
				if code != 0 {
					t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
				}
				if stderr != "" {
					t.Fatalf("stderr = %q, want empty", stderr)
				}

				assertGolden(t, format, tc.name, stdout)
			})
		}
	}
}

func TestCrossSourceOutputEquivalence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		wantRule []string
		setup    func(t *testing.T, repo string)
		change   func(t *testing.T, repo string)
	}{
		{
			name:     "db001_db002_composite",
			wantRule: []string{"DB001", "DB002"},
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n}\n")
				writeRepoFile(t, filepath.Join(repo, "prisma", "migrations", "20260330010101_drop_legacy_login", "migration.sql"), "ALTER TABLE \"User\" ADD COLUMN \"legacy_login\" TEXT;\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "schema.prisma"), "model User {\n  id Int @id\n  nickname String\n}\n")
				writeRepoFile(t, filepath.Join(repo, "prisma", "migrations", "20260330010101_drop_legacy_login", "migration.sql"), "ALTER TABLE \"User\" ADD COLUMN \"legacy_login\" TEXT;\nALTER TABLE \"User\" DROP COLUMN \"legacy_login\";\n")
			},
		},
		{
			name:     "cfg001_positive_api001_suppressed",
			wantRule: []string{"CFG001"},
			setup: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "openapi.yaml"), "paths:\n")
				writeRepoFile(t, filepath.Join(repo, "docs", "api.md"), "API\n")
				writeRepoFile(t, filepath.Join(repo, "internal", "config", "load.go"), "func load() {\n}\n")
			},
			change: func(t *testing.T, repo string) {
				writeRepoFile(t, filepath.Join(repo, "openapi.yaml"), "paths:\n  /users:\n    get:\n      summary: list users\n")
				writeRepoFile(t, filepath.Join(repo, "docs", "api.md"), "API\nDocument the new /users endpoint.\n")
				writeRepoFile(t, filepath.Join(repo, "internal", "config", "load.go"), "func load() {\n  token := os.Getenv(\"NEW_API_TOKEN\")\n  _ = token\n}\n")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newGitRepoForCheck(t)
			tc.setup(t, repo)
			commitAll(t, repo, "base")
			base := strings.TrimSpace(runGitForCheck(t, repo, "rev-parse", "HEAD"))

			tc.change(t, repo)
			workingText, workingTextCode, workingTextStderr := runCheckOutput(t, repo, []string{"--format", "text", "--fail-on", "off"})
			workingJSON, workingJSONCode, workingJSONStderr := runCheckOutput(t, repo, []string{"--format", "json", "--fail-on", "off"})

			if workingTextCode != 0 || workingTextStderr != "" {
				t.Fatalf("working text failed: code=%d stderr=%q", workingTextCode, workingTextStderr)
			}
			if workingJSONCode != 0 || workingJSONStderr != "" {
				t.Fatalf("working json failed: code=%d stderr=%q", workingJSONCode, workingJSONStderr)
			}

			commitAll(t, repo, "head")
			head := strings.TrimSpace(runGitForCheck(t, repo, "rev-parse", "HEAD"))

			rangeText, rangeTextCode, rangeTextStderr := runCheckOutput(t, repo, []string{"--base", base, "--head", head, "--format", "text", "--fail-on", "off"})
			rangeJSON, rangeJSONCode, rangeJSONStderr := runCheckOutput(t, repo, []string{"--base", base, "--head", head, "--format", "json", "--fail-on", "off"})
			if rangeTextCode != 0 || rangeTextStderr != "" {
				t.Fatalf("range text failed: code=%d stderr=%q", rangeTextCode, rangeTextStderr)
			}
			if rangeJSONCode != 0 || rangeJSONStderr != "" {
				t.Fatalf("range json failed: code=%d stderr=%q", rangeJSONCode, rangeJSONStderr)
			}

			patchPath := filepath.Join(t.TempDir(), tc.name+".diff")
			writeRepoFile(t, patchPath, runGitForCheck(t, repo, "diff", base, head))

			diffText, diffTextCode, diffTextStderr := runCheckOutput(t, repo, []string{"--diff-file", patchPath, "--format", "text", "--fail-on", "off"})
			diffJSON, diffJSONCode, diffJSONStderr := runCheckOutput(t, repo, []string{"--diff-file", patchPath, "--format", "json", "--fail-on", "off"})
			if diffTextCode != 0 || diffTextStderr != "" {
				t.Fatalf("diff-file text failed: code=%d stderr=%q", diffTextCode, diffTextStderr)
			}
			if diffJSONCode != 0 || diffJSONStderr != "" {
				t.Fatalf("diff-file json failed: code=%d stderr=%q", diffJSONCode, diffJSONStderr)
			}

			if workingText != rangeText || workingText != diffText {
				t.Fatalf("text outputs differ\nworking:\n%s\nrange:\n%s\ndiff-file:\n%s", workingText, rangeText, diffText)
			}
			if workingJSON != rangeJSON || workingJSON != diffJSON {
				t.Fatalf("json outputs differ\nworking:\n%s\nrange:\n%s\ndiff-file:\n%s", workingJSON, rangeJSON, diffJSON)
			}

			report := decodeReport(t, workingJSON)
			if got := ruleIDsFromReport(report); !equalStrings(got, tc.wantRule) {
				t.Fatalf("rule IDs = %v, want %v", got, tc.wantRule)
			}
		})
	}
}

func runCheckOutput(t *testing.T, cwd string, args []string) (string, int, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), cwd, args, &stdout, &stderr)
	return stdout.String(), code, stderr.String()
}

func decodeReport(t *testing.T, raw string) model.Report {
	t.Helper()

	var report model.Report
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw=%s", err, raw)
	}
	return report
}

func assertGolden(t *testing.T, format, name, actual string) {
	t.Helper()

	goldenPath := filepath.Join("..", "..", "testdata", "golden", format, name+".golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", goldenPath, err)
	}
	if actual != string(want) {
		t.Fatalf("golden mismatch for %s/%s\nwant:\n%s\ngot:\n%s", format, name, string(want), actual)
	}
}
