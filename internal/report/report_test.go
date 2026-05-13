package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
	"github.com/mt4110/specbackfill/internal/rules"
)

func TestWriteTextSkeleton(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{
		Files: []model.File{{Path: "dir/file.txt", Status: model.FileStatusModified}},
	}
	result := Build(model.RepoProfile{Go: true}, nil)

	if err := Write(&output, "text", diff, result); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "specbackfill check") {
		t.Fatalf("text output missing command header:\n%s", text)
	}
	if !strings.Contains(text, "changed files: 1") {
		t.Fatalf("text output missing changed file count:\n%s", text)
	}
	if !strings.Contains(text, "changed file summary:") || !strings.Contains(text, "- other: 1") {
		t.Fatalf("text output missing changed file summary:\n%s", text)
	}
	if !strings.Contains(text, "    - dir/file.txt") {
		t.Fatalf("text output missing changed file sample:\n%s", text)
	}
	if !strings.Contains(text, "No findings emitted.") {
		t.Fatalf("text output missing empty findings message:\n%s", text)
	}
}

func TestFileSummaryRows(t *testing.T) {
	t.Parallel()

	diff := model.Diff{Files: []model.File{
		{Path: "schema.prisma"},
		{Path: "prisma/migrations/20260512010101_add_user/migration.sql"},
		{Path: "openapi.yaml"},
		{Path: "generated/openapi/client.gen.ts"},
		{Path: "docs/usage.md"},
		{Path: "internal/api/handler_test.go"},
		{Path: "testdata/golden/text/db001_positive.golden"},
		{Path: ".github/workflows/ci.yaml"},
		{Path: "scripts/check.sh"},
		{Path: "internal/server/main.go"},
		{Path: "internal/server/router.go"},
		{Path: "internal/server/store.go"},
		{Path: "internal/server/worker.go"},
		{Path: "crates/domain/src/lib.rs"},
		{Path: "web/src/app.tsx"},
		{Path: "web/src/app.js"},
		{Path: "tools/load.py"},
		{Path: "queries/report.sql"},
		{Path: "examples/config.go"},
		{Path: "assets/logo.png"},
	}}

	rows := fileSummaryRows(diff)
	got := map[string]int{}
	byLabel := map[string]fileSummaryRow{}
	for _, row := range rows {
		got[row.label] = row.count
		byLabel[row.label] = row
	}

	for _, want := range []fileSummaryRow{
		{label: "db schema", count: 1},
		{label: "migrations", count: 1},
		{label: "API specs", count: 1},
		{label: "generated", count: 1},
		{label: "docs", count: 1},
		{label: "tests", count: 1},
		{label: "test fixtures", count: 1},
		{label: "config/ci", count: 1},
		{label: "scripts", count: 1},
		{label: "Go source", count: 4},
		{label: "Rust source", count: 1},
		{label: "TypeScript source", count: 1},
		{label: "JavaScript source", count: 1},
		{label: "Python source", count: 1},
		{label: "SQL", count: 1},
		{label: "examples/samples", count: 1},
		{label: "other", count: 1},
	} {
		if got[want.label] != want.count {
			t.Fatalf("summary %q = %d, want %d; rows=%+v", want.label, got[want.label], want.count, rows)
		}
	}

	goSource := byLabel["Go source"]
	if len(goSource.samples) != maxFileSummarySamples {
		t.Fatalf("Go source samples = %v, want %d samples", goSource.samples, maxFileSummarySamples)
	}
	if strings.Join(goSource.samples, ",") != "internal/server/main.go,internal/server/router.go,internal/server/store.go" {
		t.Fatalf("Go source samples = %v, want first changed files", goSource.samples)
	}
}

func TestWriteJSONSkeleton(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	result := Build(model.RepoProfile{Go: true}, nil)

	if err := Write(&output, "json", model.Diff{}, result); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["version"] != "v0" {
		t.Fatalf("version = %#v, want %q", decoded["version"], "v0")
	}
	if _, ok := decoded["summary"].(map[string]any); !ok {
		t.Fatalf("summary missing or invalid: %#v", decoded["summary"])
	}
	if findings, ok := decoded["findings"].([]any); !ok || len(findings) != 0 {
		t.Fatalf("findings = %#v, want empty array", decoded["findings"])
	}
}

func TestWriteMarkdownFinding(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{Go: true}, []model.Finding{{
		RuleID:     "DB001",
		Severity:   model.SeverityError,
		Confidence: "high",
		Title:      "Schema changed, but no matching migration companion moved with this diff",
		Why:        "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		ExpectedCompanions: []string{"migration file"},
	}})

	if err := Write(&output, "markdown", diff, result); err != nil {
		t.Fatalf("Write(markdown) error = %v", err)
	}

	markdown := output.String()
	for _, want := range []string{
		"### specbackfill findings",
		"#### [error] DB001 Schema changed, but no matching migration companion moved with this diff",
		"`schema.prisma`: `+ email String @unique`",
		"- migration file",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown output missing %q:\n%s", want, markdown)
		}
	}
}

func TestWriteMarkdownEscapesBacktickEvidence(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "internal/config/load.go", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{}, []model.Finding{{
		RuleID:     "CFG001",
		Severity:   model.SeverityWarn,
		Confidence: "high",
		Title:      "New config detected, but no matching docs/default companion moved with this diff",
		Why:        "A new config/env/flag line moved in the diff, but no matching docs/default companion evidence moved with it.",
		Evidence: []model.Evidence{{
			File:    "internal/config/load.go",
			Kind:    string(model.LineKindAdded),
			Excerpt: "token := os.Getenv(`NEW_API_TOKEN`)",
		}},
		ExpectedCompanions: []string{"docs"},
	}})

	if err := Write(&output, "markdown", diff, result); err != nil {
		t.Fatalf("Write(markdown) error = %v", err)
	}

	want := "``+ token := os.Getenv(`NEW_API_TOKEN`)``"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("markdown output missing escaped backtick evidence %q:\n%s", want, output.String())
	}
}

func TestWriteTextSummary(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{}, []model.Finding{
		{RuleID: "DB001", Severity: model.SeverityError, Evidence: []model.Evidence{{File: "schema.prisma"}}, ExpectedCompanions: []string{"migration file"}},
		{RuleID: "API001", Severity: model.SeverityWarn, Evidence: []model.Evidence{{File: "openapi.yaml"}}, ExpectedCompanions: []string{"contract test"}},
		{RuleID: "API001", Severity: model.SeverityWarn, Evidence: []model.Evidence{{File: "openapi.yaml"}}, ExpectedCompanions: []string{"contract test"}},
	})

	err := WriteWithOptions(&output, "text", diff, result, Options{SummaryOnly: true})
	if err != nil {
		t.Fatalf("WriteWithOptions(summary text) error = %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"specbackfill summary",
		"changed files: 1",
		"error: 1",
		"warn:  2",
		"info:  0",
		"- DB001: 1",
		"- API001: 2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "expected companions:") {
		t.Fatalf("summary output included finding details:\n%s", text)
	}
}

func TestWriteJSONSummary(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{Go: true}, []model.Finding{{
		RuleID:             "DB001",
		Severity:           model.SeverityError,
		Evidence:           []model.Evidence{{File: "schema.prisma"}},
		ExpectedCompanions: []string{"migration file"},
	}})

	err := WriteWithOptions(&output, "json", diff, result, Options{SummaryOnly: true})
	if err != nil {
		t.Fatalf("WriteWithOptions(summary json) error = %v", err)
	}

	var decoded struct {
		Version      string         `json:"version"`
		ChangedFiles int            `json:"changed_files"`
		Summary      model.Summary  `json:"summary"`
		RulesFired   map[string]int `json:"rules_fired"`
		Findings     []any          `json:"findings"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output.String())
	}
	if decoded.Version != "v0" || decoded.ChangedFiles != 1 || decoded.Summary.Error != 1 || decoded.RulesFired["DB001"] != 1 {
		t.Fatalf("decoded summary = %+v, want DB001 error summary", decoded)
	}
	if decoded.Findings != nil {
		t.Fatalf("summary JSON included findings: %+v", decoded.Findings)
	}
}

func TestBuildAddsStableFindingID(t *testing.T) {
	t.Parallel()

	findings := []model.Finding{{
		RuleID:     "DB001",
		Severity:   model.SeverityError,
		Confidence: "high",
		Title:      "Schema changed",
		Why:        "schema evidence moved",
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Line:    3,
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		ExpectedCompanions: []string{"migration file"},
	}}

	first := Build(model.RepoProfile{}, findings)
	second := Build(model.RepoProfile{}, findings)

	const wantID = "v0-8362254e793872c2"
	if first.Findings[0].FindingID != wantID {
		t.Fatalf("FindingID = %q, want %q", first.Findings[0].FindingID, wantID)
	}
	if first.Findings[0].FindingID != second.Findings[0].FindingID {
		t.Fatalf("FindingID not stable: first=%q second=%q", first.Findings[0].FindingID, second.Findings[0].FindingID)
	}
	if first.Findings[0].OmissionSignature != "db001.schema_changed.migration_companion" {
		t.Fatalf("OmissionSignature = %q, want DB001 signature", first.Findings[0].OmissionSignature)
	}
	if findings[0].FindingID != "" {
		t.Fatalf("Build mutated input finding: %+v", findings[0])
	}
	if findings[0].OmissionSignature != "" {
		t.Fatalf("Build mutated input omission signature: %+v", findings[0])
	}
}

func TestBuildOverwritesGeneratedFindingMetadata(t *testing.T) {
	t.Parallel()

	result := Build(model.RepoProfile{}, []model.Finding{{
		FindingID:         "non-deterministic-input-id",
		OmissionSignature: "non-deterministic-input-signature",
		RuleID:            "CFG001",
		Severity:          model.SeverityWarn,
		Confidence:        "medium",
		Title:             "New config detected",
		Why:               "config evidence moved",
		Evidence:          []model.Evidence{{File: "config.go", Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("FOO")`}},
		ExpectedCompanions: []string{
			"docs",
		},
	}})

	if got := result.Findings[0].FindingID; got == "" || got == "non-deterministic-input-id" {
		t.Fatalf("FindingID = %q, want generated deterministic ID", got)
	}
	if got := result.Findings[0].OmissionSignature; got != "cfg001.config_introduced.docs_default" {
		t.Fatalf("OmissionSignature = %q, want generated CFG001 signature", got)
	}
}

func TestBuildAddsOmissionSignatureForEveryCatalogRule(t *testing.T) {
	t.Parallel()

	for _, info := range rules.Catalog() {
		info := info
		t.Run(info.ID, func(t *testing.T) {
			t.Parallel()

			result := Build(model.RepoProfile{}, []model.Finding{{
				RuleID:             info.ID,
				Severity:           info.DefaultSeverity,
				Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
				ExpectedCompanions: []string{"companion"},
			}})

			signature := result.Findings[0].OmissionSignature
			if signature == "" {
				t.Fatalf("%s OmissionSignature is empty", info.ID)
			}
			if strings.HasSuffix(signature, ".unmapped") {
				t.Fatalf("%s OmissionSignature = %q, want explicit mapped signature", info.ID, signature)
			}
		})
	}
}

func TestBuildAddsDeterministicOmissionSignatureFallback(t *testing.T) {
	t.Parallel()

	result := Build(model.RepoProfile{}, []model.Finding{{
		RuleID:             "NEW999",
		Severity:           model.SeverityInfo,
		Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
		ExpectedCompanions: []string{"companion"},
	}})

	if got := result.Findings[0].OmissionSignature; got != "new999.unmapped" {
		t.Fatalf("OmissionSignature = %q, want deterministic fallback", got)
	}

	emptyRuleResult := Build(model.RepoProfile{}, []model.Finding{{
		Severity:           model.SeverityInfo,
		Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
		ExpectedCompanions: []string{"companion"},
	}})
	if got := emptyRuleResult.Findings[0].OmissionSignature; got != "unknown.rule_id.unmapped" {
		t.Fatalf("empty-rule OmissionSignature = %q, want unknown fallback", got)
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	findings := []model.Finding{
		{Severity: model.SeverityWarn},
		{Severity: model.SeverityError},
	}

	if code := ExitCode(findings, "error"); code != 1 {
		t.Fatalf("ExitCode(error) = %d, want 1", code)
	}
	if code := ExitCode(findings[:1], "error"); code != 0 {
		t.Fatalf("ExitCode(error without error finding) = %d, want 0", code)
	}
	if code := ExitCode(findings[:1], "warn"); code != 1 {
		t.Fatalf("ExitCode(warn) = %d, want 1", code)
	}
	if code := ExitCode(findings, "off"); code != 0 {
		t.Fatalf("ExitCode(off) = %d, want 0", code)
	}
}
