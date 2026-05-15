package todocmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenTodoOutputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fixture string
		format  string
	}{
		{name: "db001_positive", fixture: "db001_positive.diff", format: "todo_text"},
		{name: "db001_positive", fixture: "db001_positive.diff", format: "todo_markdown"},
		{name: "api001_err001_positive", fixture: "api001_err001_positive.diff", format: "todo_text"},
		{name: "db001_companion", fixture: "db001_companion.diff", format: "todo_text"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"/"+tc.format, func(t *testing.T) {
			t.Parallel()

			args := []string{"--diff-file", fixturePath(tc.fixture), "--fail-on", "off"}
			if tc.format == "todo_markdown" {
				args = append(args, "--format", "markdown")
			}
			stdout, code, stderr := runTodoOutput(t, t.TempDir(), args)
			if code != 0 {
				t.Fatalf("Run() code = %d, want 0; stderr=%q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}

			assertGolden(t, tc.format, tc.name, stdout)
		})
	}
}

func TestRunExitCodeRespectsFailOn(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want int
	}{
		{
			name: "error threshold trips on DB001",
			args: []string{"--diff-file", fixturePath("db001_positive.diff"), "--fail-on", "error"},
			want: 1,
		},
		{
			name: "off threshold stays advisory",
			args: []string{"--diff-file", fixturePath("db001_positive.diff"), "--fail-on", "off"},
			want: 0,
		},
		{
			name: "error threshold ignores warn",
			args: []string{"--diff-file", fixturePath("api001_positive.diff"), "--fail-on", "error"},
			want: 0,
		},
		{
			name: "warn threshold trips on warn",
			args: []string{"--diff-file", fixturePath("api001_positive.diff"), "--fail-on", "warn"},
			want: 1,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(context.Background(), t.TempDir(), tc.args, &stdout, &stderr)
			if code != tc.want {
				t.Fatalf("Run() code = %d, want %d; stderr=%q stdout=%q", code, tc.want, stderr.String(), stdout.String())
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidFlags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "json format",
			args: []string{"--format", "json"},
			want: `invalid --format "json"`,
		},
		{
			name: "invalid fail-on",
			args: []string{"--fail-on", "block"},
			want: `invalid --fail-on "block"`,
		},
		{
			name: "base without head",
			args: []string{"--base", "main"},
			want: "--base and --head must be provided together",
		},
		{
			name: "diff-file with range",
			args: []string{"--diff-file", "change.diff", "--base", "main", "--head", "HEAD"},
			want: "--diff-file cannot be combined with --base/--head",
		},
		{
			name: "positional",
			args: []string{"extra"},
			want: "unexpected positional arguments",
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
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
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
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage of todo:") {
		t.Fatalf("stderr = %q, want usage output", stderr.String())
	}
}

func runTodoOutput(t *testing.T, cwd string, args []string) (string, int, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), cwd, args, &stdout, &stderr)
	return stdout.String(), code, stderr.String()
}

func fixturePath(name string) string {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "patches", name))
	if err != nil {
		panic(err)
	}
	return path
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
