package explain

import (
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
)

func TestBuildGroundsExplanationInExistingFinding(t *testing.T) {
	t.Parallel()

	findings := []model.Finding{
		{
			RuleID:     "DB001",
			Severity:   model.SeverityError,
			Confidence: "high",
			Title:      "Schema changed, but no matching migration companion moved with this diff",
			Why:        "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
			Evidence: []model.Evidence{
				{File: "schema.prisma", Line: 3, Kind: string(model.LineKindAdded), Excerpt: "email String @unique"},
			},
			ExpectedCompanions: []string{"migration file"},
		},
	}

	explanations := Build(findings)
	if len(explanations) != 1 {
		t.Fatalf("len(explanations) = %d, want 1", len(explanations))
	}

	explanation := explanations[0]
	if explanation.FindingIndex != 0 {
		t.Fatalf("FindingIndex = %d, want 0", explanation.FindingIndex)
	}
	if explanation.RuleID != findings[0].RuleID {
		t.Fatalf("RuleID = %q, want %q", explanation.RuleID, findings[0].RuleID)
	}
	if explanation.Source != source {
		t.Fatalf("Source = %q, want %q", explanation.Source, source)
	}
	if len(explanation.Evidence) != len(findings[0].Evidence) {
		t.Fatalf("Evidence = %+v, want %+v", explanation.Evidence, findings[0].Evidence)
	}
	if len(explanation.ExpectedCompanions) != len(findings[0].ExpectedCompanions) {
		t.Fatalf("ExpectedCompanions = %+v, want %+v", explanation.ExpectedCompanions, findings[0].ExpectedCompanions)
	}
	if !strings.Contains(explanation.Summary, "existing DB001 finding") {
		t.Fatalf("Summary = %q, want existing finding wording", explanation.Summary)
	}
	if !strings.Contains(explanation.Summary, "schema.prisma:3:+ email String @unique") {
		t.Fatalf("Summary = %q, want evidence reference", explanation.Summary)
	}
	if strings.Contains(strings.ToLower(explanation.Summary), "migration is missing") {
		t.Fatalf("Summary violates diff-local wording: %q", explanation.Summary)
	}
}

func TestBuildWithoutFindingsReturnsNil(t *testing.T) {
	t.Parallel()

	if got := Build(nil); got != nil {
		t.Fatalf("Build(nil) = %+v, want nil", got)
	}
}
