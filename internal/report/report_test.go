package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
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
	if !strings.Contains(text, "No findings emitted.") {
		t.Fatalf("text output missing empty findings message:\n%s", text)
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
	if findings[0].FindingID != "" {
		t.Fatalf("Build mutated input finding: %+v", findings[0])
	}
}

func TestBuildOverwritesExistingFindingID(t *testing.T) {
	t.Parallel()

	result := Build(model.RepoProfile{}, []model.Finding{{
		FindingID:          "non-deterministic-input-id",
		RuleID:             "CFG001",
		Severity:           model.SeverityWarn,
		Confidence:         "medium",
		Title:              "New config detected",
		Why:                "config evidence moved",
		Evidence:           []model.Evidence{{File: "config.go", Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("FOO")`}},
		ExpectedCompanions: []string{"docs"},
	}})

	if got := result.Findings[0].FindingID; got == "" || got == "non-deterministic-input-id" {
		t.Fatalf("FindingID = %q, want generated deterministic ID", got)
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
