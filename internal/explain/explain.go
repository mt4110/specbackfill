package explain

import (
	"fmt"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

const source = "grounded-finding-explanation"

func Build(findings []model.Finding) []model.Explanation {
	if len(findings) == 0 {
		return nil
	}

	explanations := make([]model.Explanation, 0, len(findings))
	for index, finding := range findings {
		explanations = append(explanations, model.Explanation{
			FindingIndex:       index,
			RuleID:             finding.RuleID,
			Source:             source,
			Summary:            summarize(finding),
			Evidence:           append([]model.Evidence(nil), finding.Evidence...),
			ExpectedCompanions: append([]string(nil), finding.ExpectedCompanions...),
		})
	}
	return explanations
}

func summarize(finding model.Finding) string {
	parts := []string{
		fmt.Sprintf("This explains the existing %s finding: %s.", finding.RuleID, finding.Title),
	}

	if evidence := evidenceSummary(finding.Evidence); evidence != "" {
		parts = append(parts, fmt.Sprintf("The explanation is grounded in diff-local evidence: %s.", evidence))
	}
	if companions := companionSummary(finding.ExpectedCompanions); companions != "" {
		parts = append(parts, fmt.Sprintf("Expected companion categories remain: %s.", companions))
	}

	parts = append(parts, "It does not claim repository-wide absence.")
	return strings.Join(parts, " ")
}

func evidenceSummary(evidence []model.Evidence) string {
	if len(evidence) == 0 {
		return ""
	}

	items := make([]string, 0, len(evidence))
	for _, item := range evidence {
		items = append(items, formatEvidence(item))
	}
	return strings.Join(items, "; ")
}

func companionSummary(companions []string) string {
	if len(companions) == 0 {
		return ""
	}
	return strings.Join(companions, ", ")
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

	if evidence.Line > 0 {
		return fmt.Sprintf("%s:%d:%s %s", evidence.File, evidence.Line, kind, evidence.Excerpt)
	}
	return fmt.Sprintf("%s:%s %s", evidence.File, kind, evidence.Excerpt)
}
