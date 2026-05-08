package rules

import "testing"

func TestCatalogListsImplementedRulesInDefaultDisplayOrder(t *testing.T) {
	t.Parallel()

	got := Catalog()
	wantIDs := []string{"DB001", "DB002", "API001", "CFG001", "AUTH001", "ERR001", "OPS001", "DOC001"}
	if len(got) != len(wantIDs) {
		t.Fatalf("len(Catalog()) = %d, want %d", len(got), len(wantIDs))
	}

	seen := map[string]struct{}{}
	for index, info := range got {
		if info.ID != wantIDs[index] {
			t.Fatalf("Catalog()[%d].ID = %q, want %q", index, info.ID, wantIDs[index])
		}
		if _, ok := seen[info.ID]; ok {
			t.Fatalf("duplicate rule ID %q", info.ID)
		}
		seen[info.ID] = struct{}{}
		if info.DefaultSeverity == "" || info.ListDescription == "" || info.Title == "" {
			t.Fatalf("rule %s has empty display metadata: %+v", info.ID, info)
		}
		if len(info.TriggerBullets) == 0 || len(info.ExpectedCompanions) == 0 || len(info.DoesNotReportOn) == 0 {
			t.Fatalf("rule %s has empty detail metadata: %+v", info.ID, info)
		}
	}
}

func TestFindRuleInfo(t *testing.T) {
	t.Parallel()

	info, ok := FindRuleInfo("db001")
	if !ok {
		t.Fatal("FindRuleInfo(db001) ok = false, want true")
	}
	if info.ID != "DB001" {
		t.Fatalf("FindRuleInfo(db001).ID = %q, want DB001", info.ID)
	}

	if _, ok := FindRuleInfo("NOPE001"); ok {
		t.Fatal("FindRuleInfo(NOPE001) ok = true, want false")
	}
}

func TestCatalogReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	first := Catalog()
	first[0].TriggerBullets[0] = "mutated"
	first[0].ExpectedCompanions[0] = "mutated"

	second := Catalog()
	if second[0].TriggerBullets[0] == "mutated" {
		t.Fatal("Catalog() returned shared TriggerBullets storage")
	}
	if second[0].ExpectedCompanions[0] == "mutated" {
		t.Fatal("Catalog() returned shared ExpectedCompanions storage")
	}
}
