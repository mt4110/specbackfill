package rules

import (
	"path"
	"regexp"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

var prismaFieldRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s+[A-Za-z_][A-Za-z0-9_\[\]?]*`)
var termTokenRE = regexp.MustCompile(`[A-Za-z0-9_./-]+`)
var httpStatusRE = regexp.MustCompile(`\bhttp\.Status[A-Z][A-Za-z0-9_]+\b`)
var grpcCodeRE = regexp.MustCompile(`\bcodes\.[A-Z][A-Za-z0-9_]+\b`)

func Evaluate(diff model.Diff, _ model.RepoProfile) []model.Finding {
	findings := make([]model.Finding, 0, 6)

	if finding, ok := evaluateDB001(diff); ok {
		findings = append(findings, finding)
	}
	if finding, ok := evaluateDB002(diff); ok {
		findings = append(findings, finding)
	}
	if finding, ok := evaluateCFG001(diff); ok {
		findings = append(findings, finding)
	}
	if finding, ok := evaluateAPI001(diff); ok {
		findings = append(findings, finding)
	}
	if finding, ok := evaluateERR001(diff); ok {
		findings = append(findings, finding)
	}
	if finding, ok := evaluateDOC001(diff); ok {
		findings = append(findings, finding)
	}

	return findings
}

func evaluateDB001(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDB001SchemaPath(file.Path) && isChangedLine(line) && matchesDB001Line(file.Path, line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isMigrationPath, extractSearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	return model.Finding{
		RuleID:     "DB001",
		Severity:   model.SeverityError,
		Confidence: "high",
		Title:      "Schema changed, but no matching migration companion moved with this diff",
		Why:        "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"migration file",
			"migration test",
			"rollback/backfill note",
		},
	}, true
}

func evaluateDB002(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDB002TriggerPath(file.Path) && isChangedLine(line) && matchesDB002Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isDB002CompanionPath, extractSearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	return model.Finding{
		RuleID:     "DB002",
		Severity:   model.SeverityWarn,
		Confidence: "high",
		Title:      "Destructive storage change detected, but no matching rollback/backfill companion moved with this diff",
		Why:        "Destructive storage lines moved in the diff, but no matching rollback note, backfill note, or compatibility test evidence moved with them.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"rollback note",
			"data backfill note",
			"compatibility test",
		},
	}, true
}

func evaluateCFG001(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		if isCFGCompanionPath(file.Path) {
			return false
		}
		return line.Kind == model.LineKindAdded && matchesCFG001Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isCFGCompanionPath, extractSearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	confidence := "medium"
	for _, evidenceItem := range evidence {
		if looksLikeExplicitConfigKey(evidenceItem.Excerpt) {
			confidence = "high"
			break
		}
	}

	return model.Finding{
		RuleID:     "CFG001",
		Severity:   model.SeverityWarn,
		Confidence: confidence,
		Title:      "New config detected, but no matching docs/default companion moved with this diff",
		Why:        "A new config/env/flag line moved in the diff, but no matching docs/default companion evidence moved with it.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"docs",
			"default value handling",
			"upgrade note",
		},
	}, true
}

func evaluateAPI001(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isAPIPath(file.Path) && isChangedLine(line) && isMeaningfulAPILine(line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isAPICompanionPath, extractSearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	return model.Finding{
		RuleID:     "API001",
		Severity:   model.SeverityWarn,
		Confidence: "high",
		Title:      "Public API changed, but no matching contract-test/docs companion moved with this diff",
		Why:        "An explicit API spec file moved in the diff, but no matching contract-test/docs companion evidence moved with it.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"contract test",
			"API docs",
			"compatibility or deprecation note",
		},
	}, true
}

func evaluateERR001(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		if isERR001CompanionPath(file.Path) {
			return false
		}
		return isChangedLine(line) && matchesERR001Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isERR001CompanionPath, extractERR001SearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	return model.Finding{
		RuleID:     "ERR001",
		Severity:   model.SeverityWarn,
		Confidence: "high",
		Title:      "Public error/status/code contract changed, but no matching assertion-test/docs companion moved with this diff",
		Why:        "An explicit public error/status/code contract line moved in the diff, but no matching assertion-test/docs companion evidence moved with it.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"assertion test",
			"API or client note",
		},
	}, true
}

func evaluateDOC001(diff model.Diff) (model.Finding, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDOC001Path(file.Path) && isChangedLine(line) && isMeaningfulGeneratedLine(line.Text)
	})
	if len(evidence) == 0 {
		return model.Finding{}, false
	}
	if hasPositiveCompanion(diff, isDocCompanionPath, extractSearchTerms(evidence), companionTermsMatch) {
		return model.Finding{}, false
	}

	severity := model.SeverityInfo
	confidence := "medium"
	for _, evidenceItem := range evidence {
		if doc001WarnPath(evidenceItem.File) {
			severity = model.SeverityWarn
			confidence = "high"
			break
		}
	}

	return model.Finding{
		RuleID:     "DOC001",
		Severity:   severity,
		Confidence: confidence,
		Title:      "Generated spec/client changed, but no matching human-facing explanation moved with this diff",
		Why:        "A generated API/spec client artifact moved in the diff, but no matching human-facing explanation evidence moved with it.",
		Evidence:   evidence,
		ExpectedCompanions: []string{
			"human docs",
			"upgrade note",
		},
	}, true
}

func collectEvidence(diff model.Diff, limit int, match func(model.File, model.Line) bool) []model.Evidence {
	evidence := make([]model.Evidence, 0, limit)
	for _, file := range diff.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if !match(file, line) {
					continue
				}
				evidence = append(evidence, makeEvidence(file.Path, line))
				if len(evidence) == limit {
					return evidence
				}
			}
		}
	}
	return evidence
}

func makeEvidence(filePath string, line model.Line) model.Evidence {
	lineNumber := line.NewLine
	if line.Kind == model.LineKindRemoved {
		lineNumber = line.OldLine
	}

	return model.Evidence{
		File:    filePath,
		Line:    lineNumber,
		Kind:    string(line.Kind),
		Excerpt: strings.TrimSpace(line.Text),
	}
}

func isChangedLine(line model.Line) bool {
	return line.Kind == model.LineKindAdded || line.Kind == model.LineKindRemoved
}

func isDB001SchemaPath(filePath string) bool {
	switch {
	case filePath == "schema.prisma":
		return true
	case filePath == "db/schema.sql":
		return true
	case strings.HasPrefix(filePath, "ent/schema/"):
		return true
	case strings.HasPrefix(filePath, "sqlc/schema/"):
		return true
	default:
		return false
	}
}

func matchesDB001Line(filePath, text string) bool {
	if isCommentLike(text) {
		return false
	}

	upper := strings.ToUpper(text)
	for _, keyword := range []string{"CREATE TABLE", "ALTER TABLE", "DROP COLUMN", "ADD COLUMN", "CREATE INDEX"} {
		if strings.Contains(upper, keyword) {
			return true
		}
	}

	if filePath == "schema.prisma" {
		return matchesPrismaShapeChange(text)
	}
	if strings.HasPrefix(filePath, "ent/schema/") {
		return strings.Contains(text, "field.") || strings.Contains(text, "index.Fields(") || strings.Contains(text, "index.Edges(")
	}
	return false
}

func matchesPrismaShapeChange(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, prefix := range []string{"model ", "enum ", "datasource ", "generator ", "}"} {
		if strings.HasPrefix(trimmed, prefix) {
			return false
		}
	}
	if strings.Contains(trimmed, "@@index") || strings.Contains(trimmed, "@@unique") || strings.Contains(trimmed, "@@id") {
		return true
	}
	if strings.Contains(trimmed, "@id") || strings.Contains(trimmed, "@unique") {
		return true
	}
	return prismaFieldRE.MatchString(trimmed)
}

func isMigrationPath(filePath string) bool {
	return strings.HasPrefix(filePath, "migrations/") ||
		strings.HasPrefix(filePath, "db/migrations/") ||
		strings.HasPrefix(filePath, "prisma/migrations/")
}

func isDB002TriggerPath(filePath string) bool {
	return isDB001SchemaPath(filePath) || isMigrationPath(filePath)
}

func matchesDB002Line(text string) bool {
	if isCommentLike(text) {
		return false
	}

	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "DROP COLUMN"):
		return true
	case strings.Contains(upper, "DROP TABLE"):
		return true
	case strings.Contains(upper, "SET NOT NULL"):
		return true
	case strings.Contains(upper, "CREATE UNIQUE INDEX"):
		return true
	case strings.Contains(upper, "ADD CONSTRAINT") && strings.Contains(upper, "UNIQUE"):
		return true
	default:
		return false
	}
}

func isDB002CompanionPath(filePath string) bool {
	return isDocCompanionPath(filePath) || isConventionalTestPath(filePath)
}

func matchesCFG001Line(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || isCommentLike(trimmed) {
		return false
	}
	for _, trigger := range []string{"os.Getenv(", "os.LookupEnv(", "flag.", "cobra.", "viper.Get", "process.env."} {
		if strings.Contains(trimmed, trigger) {
			return true
		}
	}
	return false
}

func looksLikeExplicitConfigKey(text string) bool {
	if strings.Contains(text, `"`) || strings.Contains(text, "`") {
		return true
	}
	return strings.Contains(text, "flag.") || strings.Contains(text, "cobra.")
}

func isCFGCompanionPath(filePath string) bool {
	base := strings.ToUpper(path.Base(filePath))
	return strings.HasPrefix(base, "README") ||
		strings.HasPrefix(filePath, "docs/") ||
		filePath == ".env.example" ||
		filePath == ".env.sample" ||
		strings.HasPrefix(path.Base(filePath), "config.example") ||
		strings.HasPrefix(filePath, "examples/")
}

func isAPIPath(filePath string) bool {
	base := path.Base(filePath)
	lowerBase := strings.ToLower(base)
	lowerPath := strings.ToLower(filePath)
	switch {
	case base == "schema.graphql":
		return true
	case (strings.HasPrefix(lowerBase, "openapi") || strings.Contains(lowerPath, "/openapi/")) && (strings.HasSuffix(lowerBase, ".yaml") || strings.HasSuffix(lowerBase, ".yml")):
		return true
	case strings.HasSuffix(lowerPath, ".proto") && (strings.HasPrefix(lowerPath, "proto/") || strings.Contains(lowerPath, "/proto/")):
		return true
	default:
		return false
	}
}

func isMeaningfulAPILine(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed != "" && !isCommentLike(trimmed)
}

func isAPICompanionPath(filePath string) bool {
	if isDocCompanionPath(filePath) {
		return true
	}

	lower := strings.ToLower(filePath)
	base := strings.ToLower(path.Base(filePath))
	if strings.HasSuffix(base, "_test.go") && (strings.Contains(lower, "contract") || strings.Contains(lower, "integration") || strings.Contains(lower, "/api/")) {
		return true
	}
	return strings.Contains(lower, "/tests/contract/") || strings.Contains(lower, "/tests/integration/") || strings.Contains(lower, "/test/api/")
}

func matchesERR001Line(text string) bool {
	if isCommentLike(text) {
		return false
	}

	trimmed := strings.TrimSpace(text)
	return httpStatusRE.MatchString(trimmed) || grpcCodeRE.MatchString(trimmed)
}

func isERR001CompanionPath(filePath string) bool {
	return isDocCompanionPath(filePath) || isConventionalTestPath(filePath)
}

func isDOC001Path(filePath string) bool {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".pb.go") {
		return true
	}

	hasGeneratedSegment := hasPathSegment(lower, "generated") || hasPathSegment(lower, "gen") || hasPathSegment(lower, "client")
	if !hasGeneratedSegment {
		return false
	}

	for _, hint := range []string{"openapi", "swagger", "proto", "graphql"} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func hasPathSegment(filePath, segment string) bool {
	return filePath == segment ||
		strings.HasPrefix(filePath, segment+"/") ||
		strings.Contains(filePath, "/"+segment+"/") ||
		strings.HasSuffix(filePath, "/"+segment)
}

func doc001WarnPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".pb.go") ||
		strings.Contains(lower, "openapi") ||
		strings.Contains(lower, "swagger") ||
		strings.Contains(lower, "graphql")
}

func isMeaningfulGeneratedLine(text string) bool {
	trimmed := strings.TrimSpace(text)
	return trimmed != "" && !isCommentLike(trimmed)
}

func isDocCompanionPath(filePath string) bool {
	base := strings.ToUpper(path.Base(filePath))
	return strings.HasPrefix(filePath, "docs/") ||
		strings.HasPrefix(base, "README") ||
		strings.HasPrefix(base, "CHANGELOG") ||
		strings.HasPrefix(base, "UPGRADE")
}

func isConventionalTestPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := strings.ToLower(path.Base(filePath))

	if strings.HasSuffix(base, "_test.go") {
		return true
	}

	return strings.HasPrefix(lower, "tests/") ||
		strings.Contains(lower, "/tests/") ||
		strings.HasPrefix(lower, "integration/") ||
		strings.Contains(lower, "/integration/") ||
		strings.HasPrefix(lower, "contract/") ||
		strings.Contains(lower, "/contract/") ||
		strings.HasPrefix(lower, "e2e/") ||
		strings.Contains(lower, "/e2e/")
}

func isCommentLike(text string) bool {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range []string{"//", "#", "/*", "*/", "<!--", "--"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return trimmed == "*" || strings.HasPrefix(trimmed, "* ")
}

func hasPositiveCompanion(diff model.Diff, pathMatch func(string) bool, terms []string, lineMatch func(model.File, model.Line, []string) bool) bool {
	for _, file := range diff.Files {
		if !pathMatch(file.Path) || file.Status == model.FileStatusDeleted {
			continue
		}
		if isMetadataOnlyCompanionMove(file) {
			return true
		}
		if fileHasPositiveChange(file, terms, lineMatch) {
			return true
		}
	}
	return false
}

func isMetadataOnlyCompanionMove(file model.File) bool {
	if len(file.Hunks) != 0 {
		return false
	}

	switch file.Status {
	case model.FileStatusAdded, model.FileStatusRenamed, model.FileStatusCopied:
		return true
	default:
		return false
	}
}

func fileHasPositiveChange(file model.File, terms []string, lineMatch func(model.File, model.Line, []string) bool) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if !isChangedLine(line) || strings.TrimSpace(line.Text) == "" {
				continue
			}
			if !isDocCompanionPath(file.Path) && isCommentLike(line.Text) {
				continue
			}
			if lineMatch(file, line, terms) {
				return true
			}
		}
	}
	return false
}

func companionTermsMatch(file model.File, line model.Line, terms []string) bool {
	if len(terms) == 0 {
		return false
	}

	lowerText := strings.ToLower(line.Text)
	for _, term := range terms {
		if strings.Contains(lowerText, term) {
			return true
		}
	}
	return false
}

func extractSearchTerms(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, 8)

	for _, evidenceItem := range evidence {
		for _, source := range []string{evidenceItem.File, evidenceItem.Excerpt} {
			for _, token := range termTokenRE.FindAllString(source, -1) {
				addSearchTerms(seen, &terms, token)
			}
		}
	}

	return terms
}

func addSearchTerms(seen map[string]struct{}, terms *[]string, token string) {
	cleaned := strings.ToLower(strings.Trim(token, "\"'`:,()[]{}<>"))
	if cleaned == "" {
		return
	}

	addSearchTerm(seen, terms, cleaned)
	for _, part := range strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.'
	}) {
		addSearchTerm(seen, terms, part)
	}
	if strings.HasSuffix(cleaned, "s") && !strings.HasSuffix(cleaned, "ss") {
		addSearchTerm(seen, terms, strings.TrimSuffix(cleaned, "s"))
	}
}

func extractERR001SearchTerms(evidence []model.Evidence) []string {
	terms := extractSearchTerms(evidence)
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		seen[term] = struct{}{}
	}

	for _, evidenceItem := range evidence {
		for _, match := range httpStatusRE.FindAllString(evidenceItem.Excerpt, -1) {
			addERR001StatusTerms(seen, &terms, strings.TrimPrefix(match, "http.Status"))
		}
		for _, match := range grpcCodeRE.FindAllString(evidenceItem.Excerpt, -1) {
			addERR001StatusTerms(seen, &terms, strings.TrimPrefix(match, "codes."))
		}
	}

	return terms
}

func addERR001StatusTerms(seen map[string]struct{}, terms *[]string, raw string) {
	normalized := strings.ToLower(raw)
	addSearchTerm(seen, terms, normalized)

	words := splitCamelWords(raw)
	if len(words) == 0 {
		return
	}

	lowerWords := make([]string, 0, len(words))
	for _, word := range words {
		lowerWords = append(lowerWords, strings.ToLower(word))
	}

	joined := strings.Join(lowerWords, "")
	addSearchTerm(seen, terms, joined)
	if len(lowerWords) > 1 {
		addSearchTerm(seen, terms, strings.Join(lowerWords, " "))
		addSearchTerm(seen, terms, strings.Join(lowerWords, "_"))
		addSearchTerm(seen, terms, strings.Join(lowerWords, "-"))
	}
	if code, ok := httpStatusCodeByName[joined]; ok {
		addSearchTerm(seen, terms, code)
	}
}

func splitCamelWords(raw string) []string {
	if raw == "" {
		return nil
	}

	words := make([]string, 0, 4)
	start := 0
	for index := 1; index < len(raw); index++ {
		current := raw[index]
		prev := raw[index-1]
		if current >= 'A' && current <= 'Z' && prev >= 'a' && prev <= 'z' {
			words = append(words, raw[start:index])
			start = index
		}
	}
	words = append(words, raw[start:])
	return words
}

var httpStatusCodeByName = map[string]string{
	"badrequest":          "400",
	"unauthorized":        "401",
	"forbidden":           "403",
	"notfound":            "404",
	"conflict":            "409",
	"gone":                "410",
	"unprocessableentity": "422",
	"toomanyrequests":     "429",
	"internalservererror": "500",
	"notimplemented":      "501",
	"serviceunavailable":  "503",
}

func addSearchTerm(seen map[string]struct{}, terms *[]string, term string) {
	if len(term) < 3 {
		return
	}
	if _, blocked := ignoredSearchTerms[term]; blocked {
		return
	}
	if _, exists := seen[term]; exists {
		return
	}

	seen[term] = struct{}{}
	*terms = append(*terms, term)
}

var ignoredSearchTerms = map[string]struct{}{
	"add":        {},
	"added":      {},
	"alter":      {},
	"api":        {},
	"bool":       {},
	"change":     {},
	"client":     {},
	"column":     {},
	"config":     {},
	"create":     {},
	"delete":     {},
	"docs":       {},
	"drop":       {},
	"env":        {},
	"error":      {},
	"file":       {},
	"flag":       {},
	"generated":  {},
	"get":        {},
	"getenv":     {},
	"graphql":    {},
	"http":       {},
	"index":      {},
	"internal":   {},
	"int":        {},
	"list":       {},
	"lookupenv":  {},
	"openapi":    {},
	"return":     {},
	"migration":  {},
	"migrations": {},
	"new":        {},
	"note":       {},
	"null":       {},
	"path":       {},
	"paths":      {},
	"process":    {},
	"request":    {},
	"readme":     {},
	"response":   {},
	"schema":     {},
	"set":        {},
	"string":     {},
	"status":     {},
	"summary":    {},
	"swagger":    {},
	"table":      {},
	"token":      {},
	"proto":      {},
	"codes":      {},
	"grpc":       {},
	"unique":     {},
	"type":       {},
	"upgrade":    {},
	"warn":       {},
	"yaml":       {},
	"yml":        {},
}
