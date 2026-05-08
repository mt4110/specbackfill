package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunDispatchesRulesCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"rules", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(rules list) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "DB001") || !strings.Contains(stdout.String(), "DOC001") {
		t.Fatalf("rules list output missing rule IDs:\n%s", stdout.String())
	}
}

func TestRunDispatchesFixturesCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"fixtures", "report"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(fixtures report) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Rule    Positive  Companion  Negative  Edge") {
		t.Fatalf("fixtures report output missing header:\n%s", stdout.String())
	}
}

func TestRunDispatchesCheckCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{
		"check",
		"--diff-file", "../../testdata/patches/db001_positive.diff",
		"--fail-on", "off",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check) code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !strings.Contains(stdout.String(), "[error] DB001") {
		t.Fatalf("check output missing DB001 finding:\n%s", stdout.String())
	}
}

func TestRunRejectsUnknownTopLevelCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run(nope) code = %d, want 2", code)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "nope"`) {
		t.Fatalf("stderr missing unknown command:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "specbackfill rules list") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}

func TestRunRejectsMissingTopLevelCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run(nil) code = %d, want 2", code)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: specbackfill check [flags]") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}
