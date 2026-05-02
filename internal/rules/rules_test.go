package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mt4110/specbackfill/internal/diffparse"
	"github.com/mt4110/specbackfill/internal/model"
)

func TestEvaluateFixtures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		want []string
	}{
		{name: "db001 positive", file: "db001_positive.diff", want: []string{"DB001"}},
		{name: "db001 companion satisfied", file: "db001_companion.diff", want: nil},
		{name: "db001 deleted migration companion", file: "db001_deleted_migration.diff", want: []string{"DB001"}},
		{name: "db001 migration only", file: "db001_migration_only.diff", want: nil},
		{name: "db001 unrelated migration companion", file: "db001_unrelated_migration.diff", want: []string{"DB001"}},
		{name: "db001 ambiguous", file: "db001_ambiguous.diff", want: nil},
		{name: "db002 positive", file: "db002_positive.diff", want: []string{"DB002"}},
		{name: "db002 companion satisfied", file: "db002_companion.diff", want: nil},
		{name: "db002 removed-only companion satisfied", file: "db002_removed_companion.diff", want: nil},
		{name: "db002 paraphrased companion satisfied", file: "db002_paraphrased_companion.diff", want: nil},
		{name: "db002 deleted companion", file: "db002_deleted_companion.diff", want: []string{"DB002"}},
		{name: "db002 unrelated companion", file: "db002_unrelated_companion.diff", want: []string{"DB002"}},
		{name: "db002 additive migration negative", file: "db002_negative_additive.diff", want: nil},
		{name: "db002 ambiguous negative", file: "db002_negative_ambiguous.diff", want: nil},
		{name: "db001 and db002 composite", file: "db001_db002_positive.diff", want: []string{"DB001", "DB002"}},
		{name: "cfg001 positive", file: "cfg001_positive.diff", want: []string{"CFG001"}},
		{name: "cfg001 companion satisfied", file: "cfg001_companion.diff", want: nil},
		{name: "cfg001 deleted docs companion", file: "cfg001_deleted_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 docs only", file: "cfg001_docs_only.diff", want: nil},
		{name: "cfg001 positive with api001 suppressed", file: "cfg001_positive_api001_suppressed.diff", want: []string{"CFG001"}},
		{name: "cfg001 removed unrelated docs still warns", file: "cfg001_removed_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 unrelated docs companion", file: "cfg001_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 comment only", file: "cfg001_comment_only.diff", want: nil},
		{name: "api001 positive", file: "api001_positive.diff", want: []string{"API001"}},
		{name: "api001 and err001 composite", file: "api001_err001_positive.diff", want: []string{"API001", "ERR001"}},
		{name: "api001 companion satisfied", file: "api001_companion.diff", want: nil},
		{name: "api001 deleted docs companion", file: "api001_deleted_docs.diff", want: []string{"API001"}},
		{name: "api001 docs only", file: "api001_docs_only.diff", want: nil},
		{name: "api001 unrelated docs companion", file: "api001_unrelated_docs.diff", want: []string{"API001"}},
		{name: "err001 positive", file: "err001_positive.diff", want: []string{"ERR001"}},
		{name: "err001 companion satisfied", file: "err001_companion.diff", want: nil},
		{name: "err001 removed-only companion satisfied", file: "err001_removed_companion.diff", want: nil},
		{name: "err001 paraphrased companion satisfied", file: "err001_paraphrased_companion.diff", want: nil},
		{name: "err001 deleted companion", file: "err001_deleted_companion.diff", want: []string{"ERR001"}},
		{name: "err001 unrelated companion", file: "err001_unrelated_companion.diff", want: []string{"ERR001"}},
		{name: "err001 message only negative", file: "err001_negative_message_only.diff", want: nil},
		{name: "err001 comment only negative", file: "err001_negative_comment_only.diff", want: nil},
		{name: "api001 generated only noise guard", file: "doc001_positive.diff", want: []string{"DOC001"}},
		{name: "doc001 positive", file: "doc001_positive.diff", want: []string{"DOC001"}},
		{name: "doc001 companion satisfied", file: "doc001_companion.diff", want: nil},
		{name: "doc001 deleted docs companion", file: "doc001_deleted_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 ambiguous", file: "doc001_ambiguous.diff", want: nil},
		{name: "doc001 unrelated docs companion", file: "doc001_unrelated_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 docs only", file: "api001_docs_only.diff", want: nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diff := parseFixture(t, tc.file)
			got := ruleIDs(Evaluate(diff, model.RepoProfile{}))
			if tc.want == nil {
				tc.want = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ruleIDs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindingsIncludeRequiredFields(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"db001_positive.diff",
		"db002_positive.diff",
		"cfg001_positive.diff",
		"api001_positive.diff",
		"err001_positive.diff",
		"doc001_positive.diff",
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			findings := Evaluate(parseFixture(t, fixture), model.RepoProfile{})
			if len(findings) != 1 {
				t.Fatalf("len(findings) = %d, want 1", len(findings))
			}

			finding := findings[0]
			if finding.RuleID == "" || finding.Severity == "" || finding.Confidence == "" || finding.Title == "" || finding.Why == "" {
				t.Fatalf("finding has empty required scalar field: %+v", finding)
			}
			if len(finding.Evidence) == 0 || len(finding.Evidence) > 3 {
				t.Fatalf("len(finding.Evidence) = %d, want 1..3", len(finding.Evidence))
			}
			if len(finding.ExpectedCompanions) == 0 {
				t.Fatalf("expected companions missing: %+v", finding)
			}
		})
	}
}

func parseFixture(t *testing.T, name string) model.Diff {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "patches", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	diff, err := diffparse.Parse(data)
	if err != nil {
		t.Fatalf("Parse(%q) error = %v", name, err)
	}

	return diff
}

func ruleIDs(findings []model.Finding) []string {
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		ids = append(ids, finding.RuleID)
	}
	return ids
}
