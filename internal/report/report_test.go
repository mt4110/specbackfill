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
