package rules

import "github.com/mt4110/specbackfill/internal/model"

func ScanAnchorRuleIDs(diff model.Diff) []string {
	ruleIDs := make([]string, 0, 8)

	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isDB001SchemaPath(file.Path) && isChangedLine(line) && matchesDB001Line(file.Path, line.Text)
	}) {
		ruleIDs = append(ruleIDs, "DB001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isDB002TriggerPath(file.Path) && isChangedLine(line) && matchesDB002Line(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "DB002")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		if isCFG001SuppressedPath(file.Path) {
			return false
		}
		return line.Kind == model.LineKindAdded && matchesCFG001Line(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "CFG001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isAPI001TriggerPath(file.Path) && isChangedLine(line) && isMeaningfulAPILine(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "API001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isAUTH001TriggerPath(file.Path) && isChangedLine(line) && matchesAUTH001Line(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "AUTH001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		if isERR001SuppressedPath(file.Path) {
			return false
		}
		return isChangedLine(line) && matchesERR001Line(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "ERR001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isOPS001TriggerPath(file.Path) && isChangedLine(line) && matchesOPS001Line(file.Path, line.Text)
	}) {
		ruleIDs = append(ruleIDs, "OPS001")
	}
	if hasAnchorEvidence(diff, func(file model.File, line model.Line) bool {
		return isDOC001Path(file.Path) && isChangedLine(line) && isMeaningfulGeneratedLine(line.Text)
	}) {
		ruleIDs = append(ruleIDs, "DOC001")
	}

	return ruleIDs
}

func hasAnchorEvidence(diff model.Diff, match func(model.File, model.Line) bool) bool {
	for _, file := range diff.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if match(file, line) {
					return true
				}
			}
		}
	}
	return false
}
