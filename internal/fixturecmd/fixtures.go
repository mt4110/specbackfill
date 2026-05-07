package fixturecmd

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mt4110/specbackfill/internal/diffsrc"
	"github.com/mt4110/specbackfill/internal/fixturecov"
)

func Run(ctx context.Context, cwd string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 2
	}

	switch args[0] {
	case "report":
		if len(args) != 1 {
			return writeError(stderr, "fixtures report does not accept positional arguments")
		}
		return runReport(ctx, cwd, stdout, stderr)
	default:
		return writeError(stderr, fmt.Sprintf("unknown fixtures command %q", args[0]))
	}
}

func runReport(ctx context.Context, cwd string, stdout, stderr io.Writer) int {
	repoRoot, err := diffsrc.RepoRoot(ctx, cwd)
	if err != nil {
		return writeError(stderr, fmt.Sprintf("determine repo root: %v", err))
	}

	rows, err := fixturecov.Report(filepath.Join(repoRoot, "testdata", "patches"))
	if err != nil {
		return writeError(stderr, err.Error())
	}

	if _, err := fmt.Fprintln(stdout, "Rule    Positive  Companion  Negative  Edge"); err != nil {
		return writeError(stderr, fmt.Sprintf("render fixture report: %v", err))
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(stdout, "%-7s %-9d %-10d %-9d %d\n", row.RuleID, row.Positive, row.Companion, row.Negative, row.Edge); err != nil {
			return writeError(stderr, fmt.Sprintf("render fixture report: %v", err))
		}
	}
	return 0
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: specbackfill fixtures report")
}

func writeError(w io.Writer, message string) int {
	fmt.Fprintf(w, "error: %s\n", message)
	return 2
}
