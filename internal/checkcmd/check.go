package checkcmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/mt4110/specbackfill/internal/diffparse"
	"github.com/mt4110/specbackfill/internal/diffsrc"
	"github.com/mt4110/specbackfill/internal/explain"
	"github.com/mt4110/specbackfill/internal/profile"
	"github.com/mt4110/specbackfill/internal/report"
	"github.com/mt4110/specbackfill/internal/rules"
)

func Run(ctx context.Context, cwd string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var base string
	var head string
	var diffFile string
	var format string
	var failOn string
	var includeExplanations bool

	flags.StringVar(&base, "base", "", "base git ref")
	flags.StringVar(&head, "head", "", "head git ref")
	flags.StringVar(&diffFile, "diff-file", "", "unified diff file")
	flags.StringVar(&format, "format", "text", "output format: text|json")
	flags.StringVar(&failOn, "fail-on", "error", "threshold: error|warn|off")
	flags.BoolVar(&includeExplanations, "explain", false, "include grounded explanations for emitted findings")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if flags.NArg() != 0 {
		return writeError(stderr, "unexpected positional arguments")
	}
	if err := validateFlags(base, head, diffFile, format, failOn); err != nil {
		return writeError(stderr, err.Error())
	}

	diffRoot, profileRoot, err := resolveRoots(ctx, cwd, diffFile)
	if err != nil {
		return writeError(stderr, err.Error())
	}

	diffInput, err := loadDiff(ctx, diffRoot, base, head, diffFile)
	if err != nil {
		return writeError(stderr, err.Error())
	}

	diff, err := diffparse.Parse(diffInput)
	if err != nil {
		return writeError(stderr, err.Error())
	}

	repoProfile, err := profile.Detect(profileRoot)
	if err != nil {
		return writeError(stderr, fmt.Sprintf("detect repo profile: %v", err))
	}

	findings := rules.Evaluate(diff, repoProfile)
	result := report.Build(repoProfile, findings)
	if includeExplanations {
		result.Explanations = explain.Build(findings)
	}

	if err := report.Write(stdout, format, diff, result); err != nil {
		return writeError(stderr, fmt.Sprintf("render report: %v", err))
	}

	return report.ExitCode(result.Findings, failOn)
}

func validateFlags(base, head, diffFile, format, failOn string) error {
	switch format {
	case "text", "json":
	default:
		return fmt.Errorf("invalid --format %q", format)
	}

	switch failOn {
	case "error", "warn", "off":
	default:
		return fmt.Errorf("invalid --fail-on %q", failOn)
	}

	if diffFile != "" && (base != "" || head != "") {
		return fmt.Errorf("--diff-file cannot be combined with --base/--head")
	}
	if (base == "") != (head == "") {
		return fmt.Errorf("--base and --head must be provided together")
	}

	return nil
}

func resolveRoots(ctx context.Context, cwd, diffFile string) (string, string, error) {
	repoRoot, err := diffsrc.RepoRoot(ctx, cwd)
	if err != nil {
		if diffFile != "" {
			return cwd, cwd, nil
		}
		return "", "", err
	}

	if diffFile != "" {
		return cwd, repoRoot, nil
	}
	return repoRoot, repoRoot, nil
}

func loadDiff(ctx context.Context, cwd, base, head, diffFile string) ([]byte, error) {
	switch {
	case diffFile != "":
		return diffsrc.DiffFile(cwd, diffFile)
	case base != "" && head != "":
		return diffsrc.GitRange(ctx, cwd, base, head)
	default:
		return diffsrc.WorkingTree(ctx, cwd)
	}
}

func writeError(w io.Writer, message string) int {
	fmt.Fprintf(w, "error: %s\n", message)
	return 2
}
