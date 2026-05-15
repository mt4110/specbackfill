package todocmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/mt4110/specbackfill/internal/diffinput"
	"github.com/mt4110/specbackfill/internal/diffparse"
	"github.com/mt4110/specbackfill/internal/profile"
	"github.com/mt4110/specbackfill/internal/report"
	"github.com/mt4110/specbackfill/internal/rules"
)

func Run(ctx context.Context, cwd string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("todo", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var base string
	var head string
	var diffFile string
	var format string
	var failOn string

	flags.StringVar(&base, "base", "", "base git ref")
	flags.StringVar(&head, "head", "", "head git ref")
	flags.StringVar(&diffFile, "diff-file", "", "unified diff file")
	flags.StringVar(&format, "format", "text", "output format: text|markdown")
	flags.StringVar(&failOn, "fail-on", "error", "threshold: error|warn|off")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if flags.NArg() != 0 {
		return writeError(stderr, "unexpected positional arguments")
	}
	selection := diffinput.Selection{Base: base, Head: head, DiffFile: diffFile}
	if err := validateFlags(selection, format, failOn); err != nil {
		return writeError(stderr, err.Error())
	}

	diffRoot, profileRoot, err := diffinput.ResolveRoots(ctx, cwd, selection)
	if err != nil {
		return writeError(stderr, err.Error())
	}

	diffInput, err := diffinput.Load(ctx, diffRoot, selection)
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

	obligations := rules.EvaluateObligations(diff, repoProfile)
	findings := rules.FindingsFromObligations(obligations)

	reportOptions := report.Options{
		InputSummary: diffinput.Summary(selection),
		InputNotes:   diffinput.Notes(selection),
	}
	if err := report.WriteTodo(stdout, format, obligations, reportOptions); err != nil {
		return writeError(stderr, fmt.Sprintf("render todo: %v", err))
	}

	return report.ExitCode(findings, failOn)
}

func validateFlags(selection diffinput.Selection, format, failOn string) error {
	switch format {
	case "text", "markdown":
	default:
		return fmt.Errorf("invalid --format %q", format)
	}

	switch failOn {
	case "error", "warn", "off":
	default:
		return fmt.Errorf("invalid --fail-on %q", failOn)
	}

	return diffinput.Validate(selection)
}

func writeError(w io.Writer, message string) int {
	fmt.Fprintf(w, "error: %s\n", message)
	return 2
}
