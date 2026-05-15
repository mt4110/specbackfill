package checkcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mt4110/specbackfill/internal/diffinput"
	"github.com/mt4110/specbackfill/internal/diffparse"
	"github.com/mt4110/specbackfill/internal/explain"
	"github.com/mt4110/specbackfill/internal/profile"
	"github.com/mt4110/specbackfill/internal/report"
	"github.com/mt4110/specbackfill/internal/rules"
)

const defaultToolVersion = "v0"

type Options struct {
	ToolVersion string
}

func Run(ctx context.Context, cwd string, args []string, stdout, stderr io.Writer) int {
	return RunWithOptions(ctx, cwd, args, stdout, stderr, Options{})
}

func RunWithOptions(ctx context.Context, cwd string, args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var base string
	var head string
	var diffFile string
	var format string
	var failOn string
	var includeExplanations bool
	var summaryOnly bool
	var emitObligations bool
	var emitLocalAIReviewImport bool

	flags.StringVar(&base, "base", "", "base git ref")
	flags.StringVar(&head, "head", "", "head git ref")
	flags.StringVar(&diffFile, "diff-file", "", "unified diff file")
	flags.StringVar(&format, "format", "text", "output format: text|json|markdown")
	flags.StringVar(&failOn, "fail-on", "error", "threshold: error|warn|off")
	flags.BoolVar(&includeExplanations, "explain", false, "include grounded explanations for emitted findings")
	flags.BoolVar(&summaryOnly, "summary", false, "render summary-only output")
	flags.BoolVar(&emitObligations, "emit-obligations", false, "emit versioned companion obligation artifact JSON")
	flags.BoolVar(&emitLocalAIReviewImport, "emit-local-ai-review-import", false, "emit local-ai-review deterministic import JSONL")

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
	if err := validateFlags(selection, format, failOn, summaryOnly, includeExplanations, emitObligations, emitLocalAIReviewImport, flagWasProvided(flags, "format")); err != nil {
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
	result := report.Build(repoProfile, findings)
	if includeExplanations && !summaryOnly {
		result.Explanations = explain.Build(findings)
	}

	toolVersion := normalizeToolVersion(options.ToolVersion)
	if emitObligations {
		artifact := report.BuildObligationArtifact(report.ObligationArtifactOptions{
			ToolVersion: toolVersion,
			InputKind:   diffinput.Kind(selection),
			Base:        base,
			Head:        head,
			DiffInput:   diffInput,
		}, obligations)
		if err := report.WriteObligationArtifact(stdout, artifact); err != nil {
			return writeError(stderr, fmt.Sprintf("render obligation artifact: %v", err))
		}
		return report.ExitCode(result.Findings, failOn)
	}

	if emitLocalAIReviewImport {
		artifact := report.BuildObligationArtifact(report.ObligationArtifactOptions{
			ToolVersion: toolVersion,
			InputKind:   diffinput.Kind(selection),
			Base:        base,
			Head:        head,
			DiffInput:   diffInput,
		}, obligations)
		items := report.BuildLocalAIReviewImportItems(artifact)
		if err := report.WriteLocalAIReviewImport(stdout, items); err != nil {
			return writeError(stderr, fmt.Sprintf("render local-ai-review import: %v", err))
		}
		return report.ExitCode(result.Findings, failOn)
	}

	reportOptions := report.Options{
		SummaryOnly:         summaryOnly,
		InputSummary:        diffinput.Summary(selection),
		InputNotes:          diffinput.Notes(selection),
		AnchorScanAvailable: true,
		AnchorRuleIDs:       rules.ScanAnchorRuleIDs(diff),
	}
	if err := report.WriteWithOptions(stdout, format, diff, result, reportOptions); err != nil {
		return writeError(stderr, fmt.Sprintf("render report: %v", err))
	}

	return report.ExitCode(result.Findings, failOn)
}

func normalizeToolVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultToolVersion
	}
	return trimmed
}

func flagWasProvided(flags *flag.FlagSet, name string) bool {
	provided := false
	flags.Visit(func(visited *flag.Flag) {
		if visited.Name == name {
			provided = true
		}
	})
	return provided
}

func validateFlags(selection diffinput.Selection, format, failOn string, summaryOnly, includeExplanations, emitObligations, emitLocalAIReviewImport, formatProvided bool) error {
	switch format {
	case "text", "json", "markdown":
	default:
		return fmt.Errorf("invalid --format %q", format)
	}

	switch failOn {
	case "error", "warn", "off":
	default:
		return fmt.Errorf("invalid --fail-on %q", failOn)
	}

	if err := diffinput.Validate(selection); err != nil {
		return err
	}
	if emitObligations && summaryOnly {
		return fmt.Errorf("--emit-obligations cannot be combined with --summary")
	}
	if emitObligations && includeExplanations {
		return fmt.Errorf("--emit-obligations cannot be combined with --explain")
	}
	if emitObligations && formatProvided && format != "json" {
		return fmt.Errorf("--emit-obligations can only be combined with --format json")
	}
	if emitObligations && emitLocalAIReviewImport {
		return fmt.Errorf("--emit-obligations cannot be combined with --emit-local-ai-review-import")
	}
	if emitLocalAIReviewImport && summaryOnly {
		return fmt.Errorf("--emit-local-ai-review-import cannot be combined with --summary")
	}
	if emitLocalAIReviewImport && includeExplanations {
		return fmt.Errorf("--emit-local-ai-review-import cannot be combined with --explain")
	}
	if emitLocalAIReviewImport && formatProvided {
		return fmt.Errorf("--emit-local-ai-review-import cannot be combined with --format")
	}

	return nil
}

func writeError(w io.Writer, message string) int {
	fmt.Fprintf(w, "error: %s\n", message)
	return 2
}
