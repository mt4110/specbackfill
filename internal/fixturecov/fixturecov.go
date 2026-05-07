package fixturecov

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Row struct {
	RuleID    string
	Positive  int
	Companion int
	Negative  int
	Edge      int
}

var defaultRuleOrder = []string{"DB001", "DB002", "API001", "CFG001", "AUTH001", "ERR001", "OPS001", "DOC001"}

func Report(patchesDir string) ([]Row, error) {
	entries, err := os.ReadDir(patchesDir)
	if err != nil {
		return nil, fmt.Errorf("read fixture patches: %w", err)
	}

	rowsByRule := map[string]*Row{}
	for _, ruleID := range defaultRuleOrder {
		rowsByRule[ruleID] = &Row{RuleID: ruleID}
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".diff" {
			continue
		}

		ruleIDs, category := classifyFixture(entry.Name())
		for _, ruleID := range ruleIDs {
			row, ok := rowsByRule[ruleID]
			if !ok {
				continue
			}
			switch category {
			case "positive":
				row.Positive++
			case "companion":
				row.Companion++
			case "negative":
				row.Negative++
			default:
				row.Edge++
			}
		}
	}

	rows := make([]Row, 0, len(defaultRuleOrder))
	for _, ruleID := range defaultRuleOrder {
		rows = append(rows, *rowsByRule[ruleID])
	}
	return rows, nil
}

func classifyFixture(filename string) ([]string, string) {
	base := strings.TrimSuffix(strings.ToLower(filename), filepath.Ext(filename))
	tokens := strings.Split(base, "_")

	ruleIDs := make([]string, 0, 2)
	index := 0
	for index < len(tokens) {
		ruleID, ok := canonicalRuleToken(tokens[index])
		if !ok {
			break
		}
		ruleIDs = append(ruleIDs, ruleID)
		index++
	}
	if len(ruleIDs) == 0 {
		return nil, "edge"
	}

	rest := tokens[index:]
	switch {
	case hasAny(rest, "deleted", "removed", "unrelated", "metadata", "ambiguous"):
		return ruleIDs, "edge"
	case hasAny(rest, "companion"):
		return ruleIDs, "companion"
	case hasAny(rest, "positive"):
		return ruleIDs, "positive"
	case hasAny(rest, "negative"):
		return ruleIDs, "negative"
	case hasOnlyCategory(rest):
		return ruleIDs, "negative"
	default:
		return ruleIDs, "edge"
	}
}

func canonicalRuleToken(token string) (string, bool) {
	upper := strings.ToUpper(token)
	for _, ruleID := range defaultRuleOrder {
		if upper == ruleID {
			return ruleID, true
		}
	}
	return "", false
}

func hasAny(tokens []string, values ...string) bool {
	allowed := map[string]struct{}{}
	for _, value := range values {
		allowed[value] = struct{}{}
	}
	for _, token := range tokens {
		if _, ok := allowed[token]; ok {
			return true
		}
	}
	return false
}

func hasOnlyCategory(tokens []string) bool {
	if len(tokens) < 2 || tokens[len(tokens)-1] != "only" {
		return false
	}
	for _, token := range tokens[:len(tokens)-1] {
		switch token {
		case "docs", "tests", "generated", "examples", "samples", "migration":
			return true
		}
	}
	return false
}
