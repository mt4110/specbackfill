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

type Options struct {
	SummaryOnly bool
}

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
		withIDs[index].OmissionSignature = omissionSignature(withIDs[index].RuleID)
	}
	return withIDs
}

func omissionSignature(ruleID string) string {
	switch ruleID {
	case "DB001":
		return "db001.schema_changed.migration_companion"
	case "DB002":
		return "db002.destructive_storage.rollback_backfill"
	case "API001":
		return "api001.public_api_changed.contract_docs"
	case "CFG001":
		return "cfg001.config_introduced.docs_default"
	case "AUTH001":
		return "auth001.authz_changed.allow_deny"
	case "ERR001":
		return "err001.error_contract_changed.assertion_docs"
	case "OPS001":
		return "ops001.worker_retry_changed.runbook_observability"
	case "DOC001":
		return "doc001.generated_spec_changed.human_explanation"
	default:
		return ""
	}
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
	return WriteWithOptions(w, format, diff, result, Options{})
}

func WriteWithOptions(w io.Writer, format string, diff model.Diff, result model.Report, options Options) error {
	if options.SummaryOnly {
		return writeSummary(w, format, diff, result)
	}

	switch format {
	case "text":
		return writeText(w, diff, result)
	case "json":
		return writeJSON(w, result)
	case "markdown":
		return writeMarkdown(w, diff, result)
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

func writeMarkdown(w io.Writer, diff model.Diff, result model.Report) error {
	if _, err := fmt.Fprintln(w, "### specbackfill findings"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Findings: error=%d warn=%d info=%d\n", result.Summary.Error, result.Summary.Warn, result.Summary.Info); err != nil {
		return err
	}

	labels := result.RepoProfile.Labels()
	if len(labels) > 0 {
		if _, err := fmt.Fprintf(w, "- Repo profile: %s\n", strings.Join(labels, ", ")); err != nil {
			return err
		}
	}

	if len(result.Findings) == 0 {
		_, err := fmt.Fprintln(w, "\nNo findings emitted.")
		return err
	}

	for index, finding := range result.Findings {
		if _, err := fmt.Fprintf(w, "\n#### [%s] %s %s\n\n", finding.Severity, finding.RuleID, finding.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, finding.Why); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\nEvidence:"); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "- %s\n", formatMarkdownEvidence(evidence)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\nExpected companions:"); err != nil {
			return err
		}
		for _, companion := range finding.ExpectedCompanions {
			if _, err := fmt.Fprintf(w, "- %s\n", companion); err != nil {
				return err
			}
		}
		if explanation, ok := explanationForFinding(result.Explanations, index, finding.RuleID); ok {
			if _, err := fmt.Fprintf(w, "\nExplanation: %s\n", explanation.Summary); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSummary(w io.Writer, format string, diff model.Diff, result model.Report) error {
	switch format {
	case "text":
		return writeTextSummary(w, diff, result)
	case "json":
		return writeJSONSummary(w, diff, result)
	case "markdown":
		return writeMarkdownSummary(w, diff, result)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeTextSummary(w io.Writer, diff model.Diff, result model.Report) error {
	if _, err := fmt.Fprintln(w, "specbackfill summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changed files: %d\n\n", len(diff.Files)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "error: %d\n", result.Summary.Error); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "warn:  %d\n", result.Summary.Warn); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "info:  %d\n", result.Summary.Info); err != nil {
		return err
	}
	return writeTextRuleCounts(w, result.Findings)
}

func writeTextRuleCounts(w io.Writer, findings []model.Finding) error {
	counts, order := ruleCounts(findings)
	if len(order) == 0 {
		_, err := fmt.Fprintln(w, "\nRules fired: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "\nRules fired:"); err != nil {
		return err
	}
	for _, ruleID := range order {
		if _, err := fmt.Fprintf(w, "- %s: %d\n", ruleID, counts[ruleID]); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownSummary(w io.Writer, diff model.Diff, result model.Report) error {
	if _, err := fmt.Fprintln(w, "### specbackfill summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Error: %d\n", result.Summary.Error); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Warn: %d\n", result.Summary.Warn); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Info: %d\n", result.Summary.Info); err != nil {
		return err
	}

	counts, order := ruleCounts(result.Findings)
	if len(order) == 0 {
		_, err := fmt.Fprintln(w, "\n#### Rules fired\n\nNone.")
		return err
	}
	if _, err := fmt.Fprintln(w, "\n#### Rules fired"); err != nil {
		return err
	}
	for _, ruleID := range order {
		if _, err := fmt.Fprintf(w, "- `%s`: %d\n", ruleID, counts[ruleID]); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONSummary(w io.Writer, diff model.Diff, result model.Report) error {
	counts, _ := ruleCounts(result.Findings)
	payload := struct {
		Version      string            `json:"version"`
		ChangedFiles int               `json:"changed_files"`
		Summary      model.Summary     `json:"summary"`
		RulesFired   map[string]int    `json:"rules_fired"`
		RepoProfile  model.RepoProfile `json:"repo_profile"`
	}{
		Version:      result.Version,
		ChangedFiles: len(diff.Files),
		Summary:      result.Summary,
		RulesFired:   counts,
		RepoProfile:  result.RepoProfile,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
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

func formatMarkdownEvidence(evidence model.Evidence) string {
	kind := evidence.Kind
	switch evidence.Kind {
	case string(model.LineKindAdded):
		kind = "+"
	case string(model.LineKindRemoved):
		kind = "-"
	case string(model.LineKindContext):
		kind = "~"
	}

	return fmt.Sprintf("%s: %s", markdownCode(evidence.File), markdownCode(kind+" "+evidence.Excerpt))
}

func markdownCode(value string) string {
	longestRun := 0
	currentRun := 0
	for _, r := range value {
		if r == '`' {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
			continue
		}
		currentRun = 0
	}

	fence := strings.Repeat("`", longestRun+1)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		return fence + " " + value + " " + fence
	}
	return fence + value + fence
}

func ruleCounts(findings []model.Finding) (map[string]int, []string) {
	counts := map[string]int{}
	order := make([]string, 0, len(findings))
	for _, finding := range findings {
		if _, ok := counts[finding.RuleID]; !ok {
			order = append(order, finding.RuleID)
		}
		counts[finding.RuleID]++
	}
	return counts, order
}
