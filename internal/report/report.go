package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

func Build(repoProfile model.RepoProfile, findings []model.Finding) model.Report {
	if findings == nil {
		findings = []model.Finding{}
	}

	return model.Report{
		Version:     "v0",
		Summary:     model.SummaryFromFindings(findings),
		RepoProfile: repoProfile,
		Findings:    findings,
	}
}

func Write(w io.Writer, format string, diff model.Diff, result model.Report) error {
	switch format {
	case "text":
		return writeText(w, diff, result)
	case "json":
		return writeJSON(w, result)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func ExitCode(findings []model.Finding, failOn string) int {
	switch failOn {
	case "off":
		return 0
	case "warn":
		for _, finding := range findings {
			if finding.Severity == model.SeverityError || finding.Severity == model.SeverityWarn {
				return 1
			}
		}
		return 0
	case "error":
		for _, finding := range findings {
			if finding.Severity == model.SeverityError {
				return 1
			}
		}
		return 0
	default:
		return 2
	}
}

func writeText(w io.Writer, diff model.Diff, result model.Report) error {
	if _, err := fmt.Fprintln(w, "specbackfill check"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "findings: error=%d warn=%d info=%d\n", result.Summary.Error, result.Summary.Warn, result.Summary.Info); err != nil {
		return err
	}

	labels := result.RepoProfile.Labels()
	if len(labels) > 0 {
		if _, err := fmt.Fprintf(w, "repo profile: %s\n", strings.Join(labels, ", ")); err != nil {
			return err
		}
	}

	if len(result.Findings) == 0 {
		_, err := fmt.Fprintln(w, "No findings emitted.")
		return err
	}

	for index, finding := range result.Findings {
		if index > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "[%s] %s %s\n", finding.Severity, finding.RuleID, finding.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  why: %s\n", finding.Why); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "  evidence:"); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "    - %s\n", formatEvidence(evidence)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "  expected companions:"); err != nil {
			return err
		}
		for _, companion := range finding.ExpectedCompanions {
			if _, err := fmt.Fprintf(w, "    - %s\n", companion); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeJSON(w io.Writer, result model.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func formatEvidence(evidence model.Evidence) string {
	kind := evidence.Kind
	switch evidence.Kind {
	case string(model.LineKindAdded):
		kind = "+"
	case string(model.LineKindRemoved):
		kind = "-"
	case string(model.LineKindContext):
		kind = "~"
	}

	return fmt.Sprintf("%s:%s %s", evidence.File, kind, evidence.Excerpt)
}
