package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

func Build(repoProfile model.RepoProfile, findings []model.Finding) model.Report {
	if findings == nil {
		findings = []model.Finding{}
	}
	findings = withFindingIDs(findings)

	return model.Report{
		Version:     "v0",
		Summary:     model.SummaryFromFindings(findings),
		RepoProfile: repoProfile,
		Findings:    findings,
	}
}

func withFindingIDs(findings []model.Finding) []model.Finding {
	withIDs := make([]model.Finding, len(findings))
	copy(withIDs, findings)
	for index := range withIDs {
		withIDs[index].FindingID = stableFindingID(withIDs[index])
	}
	return withIDs
}

func stableFindingID(finding model.Finding) string {
	hash := sha256.New()
	writeFingerprintField(hash, "version", "specbackfill-finding-v0")
	writeFingerprintField(hash, "rule_id", finding.RuleID)
	writeFingerprintField(hash, "evidence_count", strconv.Itoa(len(finding.Evidence)))
	for index, evidence := range finding.Evidence {
		prefix := fmt.Sprintf("evidence.%d.", index)
		writeFingerprintField(hash, prefix+"file", evidence.File)
		writeFingerprintField(hash, prefix+"line", strconv.Itoa(evidence.Line))
		writeFingerprintField(hash, prefix+"kind", evidence.Kind)
		writeFingerprintField(hash, prefix+"excerpt", evidence.Excerpt)
	}
	writeFingerprintField(hash, "expected_companion_count", strconv.Itoa(len(finding.ExpectedCompanions)))
	for index, companion := range finding.ExpectedCompanions {
		writeFingerprintField(hash, fmt.Sprintf("expected_companion.%d", index), companion)
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))
	return "v0-" + sum[:16]
}

func writeFingerprintField(w io.Writer, name, value string) {
	_, _ = io.WriteString(w, name)
	_, _ = io.WriteString(w, "\x00")
	_, _ = io.WriteString(w, value)
	_, _ = io.WriteString(w, "\x00")
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
		if explanation, ok := explanationForFinding(result.Explanations, index, finding.RuleID); ok {
			if _, err := fmt.Fprintf(w, "  explanation: %s\n", explanation.Summary); err != nil {
				return err
			}
		}
	}

	return nil
}

func explanationForFinding(explanations []model.Explanation, index int, ruleID string) (model.Explanation, bool) {
	for _, explanation := range explanations {
		if explanation.FindingIndex == index && explanation.RuleID == ruleID {
			return explanation, true
		}
	}
	return model.Explanation{}, false
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
