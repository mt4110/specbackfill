package fixturecov

import (
	"path/filepath"
	"testing"
)

func TestReportCoversImplementedRules(t *testing.T) {
	t.Parallel()

	rows, err := Report(filepath.Join("..", "..", "testdata", "patches"))
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if len(rows) != len(defaultRuleOrder) {
		t.Fatalf("len(rows) = %d, want %d", len(rows), len(defaultRuleOrder))
	}

	for _, row := range rows {
		if row.Positive == 0 {
			t.Fatalf("%s has no positive fixture coverage: %+v", row.RuleID, row)
		}
		if row.Negative == 0 {
			t.Fatalf("%s has no negative fixture coverage: %+v", row.RuleID, row)
		}
	}
}

func TestClassifyFixture(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		wantRuleIDs  []string
		wantCategory string
	}{
		{name: "db001_positive.diff", wantRuleIDs: []string{"DB001"}, wantCategory: "positive"},
		{name: "db001_db002_positive.diff", wantRuleIDs: []string{"DB001", "DB002"}, wantCategory: "positive"},
		{name: "api001_err001_positive.diff", wantRuleIDs: []string{"API001", "ERR001"}, wantCategory: "positive"},
		{name: "api001_companion.diff", wantRuleIDs: []string{"API001"}, wantCategory: "companion"},
		{name: "api001_unrelated_docs.diff", wantRuleIDs: []string{"API001"}, wantCategory: "edge"},
		{name: "cfg001_negative_samples_only.diff", wantRuleIDs: []string{"CFG001"}, wantCategory: "negative"},
		{name: "db001_migration_only.diff", wantRuleIDs: []string{"DB001"}, wantCategory: "negative"},
		{name: "db001_docs_samples_only.diff", wantRuleIDs: []string{"DB001"}, wantCategory: "negative"},
		{name: "db001_docs_garbage_only.diff", wantRuleIDs: []string{"DB001"}, wantCategory: "edge"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotRuleIDs, gotCategory := classifyFixture(tc.name)
			if !equalStrings(gotRuleIDs, tc.wantRuleIDs) {
				t.Fatalf("rule IDs = %v, want %v", gotRuleIDs, tc.wantRuleIDs)
			}
			if gotCategory != tc.wantCategory {
				t.Fatalf("category = %q, want %q", gotCategory, tc.wantCategory)
			}
		})
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
