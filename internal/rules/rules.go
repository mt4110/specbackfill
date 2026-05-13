package rules

import (
	"path"
	"regexp"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

var prismaFieldRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s+[A-Za-z_][A-Za-z0-9_\[\]?]*`)
var termTokenRE = regexp.MustCompile(`[A-Za-z0-9_./-]+`)
var alnumTokenRE = regexp.MustCompile(`[a-z0-9]+`)
var httpStatusRE = regexp.MustCompile(`\bhttp\.Status[A-Z][A-Za-z0-9_]+\b`)
var grpcCodeRE = regexp.MustCompile(`\bcodes\.[A-Z][A-Za-z0-9_]+\b`)
var durationLiteralRE = regexp.MustCompile(`\b\d+\s*(ms|s|m|h)\b`)
var cronExpressionRE = regexp.MustCompile(`(?i)["'](?:@(?:annually|yearly|monthly|weekly|daily|midnight|hourly|reboot)|(?:[a-z0-9*/?,lw#-]+\s+){4,6}[a-z0-9*/?,lw#-]+)["']`)

func Evaluate(diff model.Diff, repoProfile model.RepoProfile) []model.Finding {
	return FindingsFromObligations(EvaluateObligations(diff, repoProfile))
}

func EvaluateObligations(diff model.Diff, _ model.RepoProfile) []model.Obligation {
	obligations := make([]model.Obligation, 0, 8)

	if obligation, ok := evaluateDB001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateDB002(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateCFG001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateAPI001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateAUTH001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateERR001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateOPS001(diff); ok {
		obligations = append(obligations, obligation)
	}
	if obligation, ok := evaluateDOC001(diff); ok {
		obligations = append(obligations, obligation)
	}

	return obligations
}

func FindingsFromObligations(obligations []model.Obligation) []model.Finding {
	findings := make([]model.Finding, 0, len(obligations))
	for _, obligation := range obligations {
		if obligation.Status != model.ObligationStatusMissing {
			continue
		}
		findings = append(findings, model.Finding{
			RuleID:             obligation.RuleID,
			Severity:           obligation.Severity,
			Confidence:         obligation.Confidence,
			Title:              obligation.Title,
			Why:                obligation.Why,
			Evidence:           obligation.Evidence,
			ExpectedCompanions: obligation.ExpectedCompanions,
		})
	}
	return findings
}

type obligationSpec struct {
	ruleID             string
	severity           model.Severity
	confidence         string
	title              string
	why                string
	satisfiedTitle     string
	satisfiedWhy       string
	anchorKind         string
	companionKind      string
	expectedPaths      []string
	expectedCompanions []string
}

func buildObligation(spec obligationSpec, evidence []model.Evidence, status model.ObligationStatus, satisfiers []string) model.Obligation {
	var line *int
	if len(evidence) > 0 && evidence[0].Line > 0 {
		lineValue := evidence[0].Line
		line = &lineValue
	}

	anchor := model.ObligationAnchor{
		Kind:     spec.anchorKind,
		Evidence: evidence,
	}
	if len(evidence) > 0 {
		anchor.Path = evidence[0].File
		anchor.Line = line
	}

	if satisfiers == nil {
		satisfiers = []string{}
	}

	title := spec.title
	why := spec.why
	if status == model.ObligationStatusSatisfied {
		if spec.satisfiedTitle != "" {
			title = spec.satisfiedTitle
		}
		if spec.satisfiedWhy != "" {
			why = spec.satisfiedWhy
		}
	}

	return model.Obligation{
		RuleID:         spec.ruleID,
		RuleVersion:    "v0",
		Status:         status,
		Severity:       spec.severity,
		Confidence:     spec.confidence,
		Title:          title,
		Why:            why,
		DiffLocalClaim: true,
		Anchor:         anchor,
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          spec.companionKind,
			Status:        status,
			Satisfiers:    satisfiers,
			ExpectedPaths: spec.expectedPaths,
		}},
		Evidence: evidence,
		Downstream: model.DownstreamMetadata{
			ImportKind:   "deterministic_static_layer",
			SourceSignal: "specbackfill",
		},
		ExpectedCompanions: spec.expectedCompanions,
	}
}

func obligationStatusFromSatisfiers(satisfiers []string) model.ObligationStatus {
	if len(satisfiers) > 0 {
		return model.ObligationStatusSatisfied
	}
	return model.ObligationStatusMissing
}

func evaluateDB001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDB001SchemaPath(file.Path) && isChangedLine(line) && matchesDB001Line(file.Path, line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isMigrationPath, extractSearchTerms(evidence), companionTermsMatch)
	spec := obligationSpec{
		ruleID:         "DB001",
		severity:       model.SeverityError,
		confidence:     "high",
		title:          "Schema changed, but no matching migration companion moved with this diff",
		why:            "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		satisfiedTitle: "Schema changed and matching migration companion moved with this diff",
		satisfiedWhy:   "Schema-affecting lines moved in the diff, and matching migration companion evidence moved with them.",
		anchorKind:     "schema_change",
		companionKind:  "migration_companion",
		expectedPaths: []string{
			"migrations/**",
			"db/migrations/**",
			"prisma/migrations/**",
		},
		expectedCompanions: []string{
			"migration file",
			"migration test",
			"rollback/backfill note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateDB002(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDB002TriggerPath(file.Path) && isChangedLine(line) && matchesDB002Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isDB002CompanionPath, extractSearchTerms(evidence), companionTermsMatch)
	spec := obligationSpec{
		ruleID:         "DB002",
		severity:       model.SeverityWarn,
		confidence:     "high",
		title:          "Destructive storage change detected, but no matching rollback/backfill companion moved with this diff",
		why:            "Destructive storage lines moved in the diff, but no matching rollback note, backfill note, or compatibility test evidence moved with them.",
		satisfiedTitle: "Destructive storage change detected and matching rollback/backfill companion moved with this diff",
		satisfiedWhy:   "Destructive storage lines moved in the diff, and matching rollback, backfill, or compatibility-test evidence moved with them.",
		anchorKind:     "destructive_storage_change",
		companionKind:  "rollback_backfill_companion",
		expectedPaths: []string{
			"docs/**",
			"README*",
			"CHANGELOG*",
			"UPGRADE*",
			"tests/**",
			"**/*_test.go",
		},
		expectedCompanions: []string{
			"rollback note",
			"data backfill note",
			"compatibility test",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateCFG001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		if isCFG001SuppressedPath(file.Path) {
			return false
		}
		return line.Kind == model.LineKindAdded && matchesCFG001Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isCFGCompanionPath, extractSearchTerms(evidence), companionTermsMatch)

	confidence := "medium"
	for _, evidenceItem := range evidence {
		if looksLikeExplicitConfigKey(evidenceItem.Excerpt) {
			confidence = "high"
			break
		}
	}

	spec := obligationSpec{
		ruleID:         "CFG001",
		severity:       model.SeverityWarn,
		confidence:     confidence,
		title:          "New config detected, but no matching docs/default companion moved with this diff",
		why:            "A new config/env/flag line moved in the diff, but no matching docs/default companion evidence moved with it.",
		satisfiedTitle: "New config detected and matching docs/default companion moved with this diff",
		satisfiedWhy:   "A new config/env/flag line moved in the diff, and matching docs/default companion evidence moved with it.",
		anchorKind:     "config_introduction",
		companionKind:  "config_docs_default_companion",
		expectedPaths: []string{
			"README*",
			"docs/**",
			".env.example",
			".env.sample",
			"config.example*",
			"examples/**",
			"sample/**",
			"samples/**",
		},
		expectedCompanions: []string{
			"docs",
			"default value handling",
			"upgrade note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateAPI001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isAPI001TriggerPath(file.Path) && isChangedLine(line) && isMeaningfulAPILine(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isAPICompanionPath, extractSearchTerms(evidence), companionTermsMatch)
	spec := obligationSpec{
		ruleID:         "API001",
		severity:       model.SeverityWarn,
		confidence:     "high",
		title:          "Public API changed, but no matching contract-test/docs companion moved with this diff",
		why:            "An explicit API spec file moved in the diff, but no matching contract-test/docs companion evidence moved with it.",
		satisfiedTitle: "Public API changed and matching contract-test/docs companion moved with this diff",
		satisfiedWhy:   "An explicit API spec file moved in the diff, and matching contract-test/docs companion evidence moved with it.",
		anchorKind:     "public_api_change",
		companionKind:  "contract_docs_companion",
		expectedPaths: []string{
			"docs/**",
			"README*",
			"CHANGELOG*",
			"UPGRADE*",
			"tests/contract/**",
			"tests/integration/**",
			"**/*_test.go",
		},
		expectedCompanions: []string{
			"contract test",
			"API docs",
			"compatibility or deprecation note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateAUTH001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isAUTH001TriggerPath(file.Path) && isChangedLine(line) && matchesAUTH001Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	context := buildAUTH001CompanionContext(evidence)
	satisfiers := collectAUTH001CompanionPaths(diff, context)
	spec := obligationSpec{
		ruleID:         "AUTH001",
		severity:       model.SeverityWarn,
		confidence:     "high",
		title:          "Authn/Authz branch changed, but no matching allow/deny or security-sensitive note companion moved with this diff",
		why:            "Authorization-sensitive lines moved in the diff, but no matching allow/deny test or security-sensitive note evidence moved with them.",
		satisfiedTitle: "Authn/Authz branch changed and matching allow/deny or security-sensitive note companion moved with this diff",
		satisfiedWhy:   "Authorization-sensitive lines moved in the diff, and matching allow/deny test or security-sensitive note evidence moved with them.",
		anchorKind:     "authz_branch_change",
		companionKind:  "allow_deny_security_companion",
		expectedPaths: []string{
			"tests/**",
			"**/*_test.go",
			"docs/**",
			"security.md",
		},
		expectedCompanions: []string{
			"allow test",
			"deny test",
			"security-sensitive note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateERR001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		if isERR001SuppressedPath(file.Path) {
			return false
		}
		return isChangedLine(line) && matchesERR001Line(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isERR001CompanionPath, extractERR001SearchTerms(evidence), companionTermsMatch)
	spec := obligationSpec{
		ruleID:         "ERR001",
		severity:       model.SeverityWarn,
		confidence:     "high",
		title:          "Public error/status/code contract changed, but no matching assertion-test/docs companion moved with this diff",
		why:            "An explicit public error/status/code contract line moved in the diff, but no matching assertion-test/docs companion evidence moved with it.",
		satisfiedTitle: "Public error/status/code contract changed and matching assertion-test/docs companion moved with this diff",
		satisfiedWhy:   "An explicit public error/status/code contract line moved in the diff, and matching assertion-test/docs companion evidence moved with it.",
		anchorKind:     "error_contract_change",
		companionKind:  "assertion_docs_companion",
		expectedPaths: []string{
			"tests/**",
			"**/*_test.go",
			"docs/**",
			"README*",
			"CHANGELOG*",
			"UPGRADE*",
		},
		expectedCompanions: []string{
			"assertion test",
			"API or client note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateOPS001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isOPS001TriggerPath(file.Path) && isChangedLine(line) && matchesOPS001Line(file.Path, line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	context := buildOPS001CompanionContext(evidence)
	satisfiers := collectOPS001CompanionPaths(diff, context)

	spec := obligationSpec{
		ruleID:         "OPS001",
		severity:       model.SeverityWarn,
		confidence:     "high",
		title:          "Worker/queue/retry behavior changed, but no matching observability/runbook companion moved with this diff",
		why:            "Operational worker/queue/retry lines moved in the diff, but no matching observability, runbook, or rollback guidance evidence moved with them.",
		satisfiedTitle: "Worker/queue/retry behavior changed and matching observability/runbook companion moved with this diff",
		satisfiedWhy:   "Operational worker/queue/retry lines moved in the diff, and matching observability, runbook, or rollback guidance evidence moved with them.",
		anchorKind:     "ops_behavior_change",
		companionKind:  "observability_runbook_companion",
		expectedPaths: []string{
			"docs/**",
			"runbooks/**",
			"playbooks/**",
			"observability/**",
			"monitoring/**",
			"dashboards/**",
		},
		expectedCompanions: []string{
			"observability note",
			"runbook update",
			"rollback path",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
}

func evaluateDOC001(diff model.Diff) (model.Obligation, bool) {
	evidence := collectEvidence(diff, 3, func(file model.File, line model.Line) bool {
		return isDOC001Path(file.Path) && isChangedLine(line) && isMeaningfulGeneratedLine(line.Text)
	})
	if len(evidence) == 0 {
		return model.Obligation{}, false
	}
	satisfiers := collectPositiveCompanionPaths(diff, isDocCompanionPath, extractSearchTerms(evidence), companionTermsMatch)

	severity := model.SeverityInfo
	confidence := "medium"
	for _, evidenceItem := range evidence {
		if doc001WarnPath(evidenceItem.File) {
			severity = model.SeverityWarn
			confidence = "high"
			break
		}
	}

	spec := obligationSpec{
		ruleID:         "DOC001",
		severity:       severity,
		confidence:     confidence,
		title:          "Generated spec/client changed, but no matching human-facing explanation moved with this diff",
		why:            "A generated API/spec client artifact moved in the diff, but no matching human-facing explanation evidence moved with it.",
		satisfiedTitle: "Generated spec/client changed and matching human-facing explanation moved with this diff",
		satisfiedWhy:   "A generated API/spec client artifact moved in the diff, and matching human-facing explanation evidence moved with it.",
		anchorKind:     "generated_spec_change",
		companionKind:  "human_explanation_companion",
		expectedPaths: []string{
			"docs/**",
			"README*",
			"CHANGELOG*",
			"UPGRADE*",
		},
		expectedCompanions: []string{
			"human docs",
			"upgrade note",
		},
	}
	return buildObligation(spec, evidence, obligationStatusFromSatisfiers(satisfiers), satisfiers), true
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
		isExamplePath(filePath)
}

func isCFG001SuppressedPath(filePath string) bool {
	return isCFGCompanionPath(filePath) || isGeneratedPath(filePath) || isConventionalTestPath(filePath)
}

func isAPI001TriggerPath(filePath string) bool {
	if isGeneratedPath(filePath) || isConventionalTestPath(filePath) || isExamplePath(filePath) {
		return false
	}
	return isAPIPath(filePath)
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

func isAUTH001TriggerPath(filePath string) bool {
	if isGeneratedPath(filePath) || isDocCompanionPath(filePath) || isConventionalTestPath(filePath) || isExamplePath(filePath) {
		return false
	}

	lower := strings.ToLower(filePath)
	for _, segment := range []string{
		"auth",
		"authn",
		"authz",
		"authorization",
		"authentication",
		"permission",
		"permissions",
		"rbac",
		"acl",
		"middleware",
		"middlewares",
		"guard",
		"guards",
		"policy",
		"policies",
	} {
		if hasPathSegment(lower, segment) {
			return true
		}
	}

	base := strings.ToLower(path.Base(filePath))
	for _, hint := range []string{"auth", "permission", "middleware", "guard", "policy"} {
		if strings.Contains(base, hint) {
			return true
		}
	}
	return false
}

func matchesAUTH001Line(text string) bool {
	if isCommentLike(text) {
		return false
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	for _, trigger := range []string{
		"authorize",
		"authorization",
		"authz",
		"authn",
		"permission",
		"role",
		"scope",
		"forbidden",
		"unauthorized",
		"unauthenticated",
		"middleware",
		"guard",
		"policy",
	} {
		if strings.Contains(lower, trigger) {
			return true
		}
	}
	return false
}

type auth001CompanionContext struct {
	terms             []string
	fallbackPathTerms []string
	evidencePaths     []string
}

func hasAUTH001Companion(diff model.Diff, context auth001CompanionContext) bool {
	return len(collectAUTH001CompanionPaths(diff, context)) > 0
}

func collectAUTH001CompanionPaths(diff model.Diff, context auth001CompanionContext) []string {
	paths := make([]string, 0, 2)
	hasAllow := false
	hasDeny := false
	allowPaths := make([]string, 0, 1)
	denyPaths := make([]string, 0, 1)

	for _, file := range diff.Files {
		if file.Status == model.FileStatusDeleted {
			continue
		}
		if isAUTH001MetadataCompanionMove(file, context) {
			paths = appendUniquePath(paths, file.Path)
			continue
		}
		if isAUTH001SecurityNotePath(file.Path) && fileHasAUTH001SecurityNote(file, context) {
			paths = appendUniquePath(paths, file.Path)
			continue
		}
		if !isAUTH001TestPath(file.Path) {
			continue
		}
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind != model.LineKindAdded || strings.TrimSpace(line.Text) == "" || isCommentLike(line.Text) {
					continue
				}
				if !matchesAUTH001CompanionContext(file, line, context) {
					continue
				}
				if isAUTH001AllowLine(line.Text) {
					hasAllow = true
					allowPaths = appendUniquePath(allowPaths, file.Path)
				}
				if isAUTH001DenyLine(line.Text) {
					hasDeny = true
					denyPaths = appendUniquePath(denyPaths, file.Path)
				}
			}
		}
	}

	if hasAllow && hasDeny {
		for _, path := range allowPaths {
			paths = appendUniquePath(paths, path)
		}
		for _, path := range denyPaths {
			paths = appendUniquePath(paths, path)
		}
	}

	return paths
}

func buildAUTH001CompanionContext(evidence []model.Evidence) auth001CompanionContext {
	return auth001CompanionContext{
		terms:             extractAUTH001SearchTerms(evidence),
		fallbackPathTerms: extractAUTH001FallbackPathTerms(evidence),
		evidencePaths:     extractAUTH001EvidencePaths(evidence),
	}
}

func extractAUTH001SearchTerms(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, 8)

	for _, evidenceItem := range evidence {
		for _, token := range termTokenRE.FindAllString(evidenceItem.Excerpt, -1) {
			addAUTH001SearchTerms(seen, &terms, token)
		}
	}

	return terms
}

func addAUTH001SearchTerms(seen map[string]struct{}, terms *[]string, token string) {
	cleaned := strings.ToLower(strings.Trim(token, "\"'`:,()[]{}<>"))
	if cleaned == "" {
		return
	}

	addAUTH001SearchTerm(seen, terms, cleaned)
	for _, part := range strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.'
	}) {
		addAUTH001SearchTerm(seen, terms, part)
	}
	if strings.HasSuffix(cleaned, "s") && !strings.HasSuffix(cleaned, "ss") {
		addAUTH001SearchTerm(seen, terms, strings.TrimSuffix(cleaned, "s"))
	}
}

func addAUTH001SearchTerm(seen map[string]struct{}, terms *[]string, term string) {
	if _, blocked := ignoredAUTH001SearchTerms[term]; blocked {
		return
	}
	addSearchTerm(seen, terms, term)
}

func extractAUTH001FallbackPathTerms(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(evidence))

	for _, evidenceItem := range evidence {
		base := strings.ToLower(path.Base(evidenceItem.File))
		if ext := path.Ext(base); ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
		addAUTH001FallbackPathTerm(seen, &terms, base)
		for _, segment := range strings.Split(strings.ToLower(path.Dir(evidenceItem.File)), "/") {
			addAUTH001FallbackPathTerm(seen, &terms, segment)
		}
	}

	return terms
}

func extractAUTH001EvidencePaths(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	paths := make([]string, 0, len(evidence))

	for _, evidenceItem := range evidence {
		normalized := strings.ToLower(evidenceItem.File)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		paths = append(paths, normalized)
	}

	return paths
}

func addAUTH001FallbackPathTerm(seen map[string]struct{}, terms *[]string, term string) {
	if len(term) < 3 {
		return
	}
	if _, blocked := ignoredAUTH001FallbackPathTerms[term]; blocked {
		return
	}
	if _, exists := seen[term]; exists {
		return
	}

	seen[term] = struct{}{}
	*terms = append(*terms, term)
}

var ignoredAUTH001SearchTerms = map[string]struct{}{
	"auth":           {},
	"authentication": {},
	"authorization":  {},
	"authorize":      {},
	"authn":          {},
	"authz":          {},
	"forbidden":      {},
	"guard":          {},
	"middleware":     {},
	"permission":     {},
	"permissions":    {},
	"policy":         {},
	"role":           {},
	"roles":          {},
	"scope":          {},
	"scopes":         {},
	"unauthorized":   {},
	"user":           {},
	"users":          {},
}

var ignoredAUTH001FallbackPathTerms = map[string]struct{}{
	"acl":            {},
	"auth":           {},
	"authentication": {},
	"authorization":  {},
	"authn":          {},
	"authz":          {},
	"internal":       {},
	"middleware":     {},
	"middlewares":    {},
	"permission":     {},
	"permissions":    {},
	"policy":         {},
	"policies":       {},
	"rbac":           {},
}

func matchesAUTH001CompanionContext(file model.File, line model.Line, context auth001CompanionContext) bool {
	if companionTermsMatch(file, line, context.terms) {
		return true
	}
	if isAUTH001EmptyContext(context) {
		if isAUTH001SecurityNotePath(file.Path) {
			return true
		}
		return isAUTH001RelatedTestCompanionPath(strings.ToLower(file.Path), context)
	}

	lowerPath := strings.ToLower(file.Path)
	lowerText := strings.ToLower(line.Text)
	for _, term := range context.fallbackPathTerms {
		if strings.Contains(lowerPath, term) || strings.Contains(lowerText, term) {
			return true
		}
	}
	return false
}

func isAUTH001EmptyContext(context auth001CompanionContext) bool {
	return len(context.terms) == 0 && len(context.fallbackPathTerms) == 0
}

func isAUTH001MetadataCompanionMove(file model.File, context auth001CompanionContext) bool {
	if !isMetadataOnlyCompanionMove(file) {
		return false
	}
	if isAUTH001SecurityNotePath(file.Path) {
		return true
	}
	return isAUTH001RelatedTestCompanionPath(strings.ToLower(file.Path), context)
}

func isAUTH001RelatedTestCompanionPath(filePath string, context auth001CompanionContext) bool {
	if !isAUTH001TestPath(filePath) {
		return false
	}

	for _, term := range context.terms {
		if strings.Contains(filePath, term) {
			return true
		}
	}
	for _, term := range context.fallbackPathTerms {
		if strings.Contains(filePath, term) {
			return true
		}
	}

	companionDir := path.Dir(filePath)
	companionBase := strings.TrimSuffix(path.Base(filePath), path.Ext(filePath))
	companionBase = strings.TrimSuffix(companionBase, "_test")

	for _, evidencePath := range context.evidencePaths {
		evidenceDir := path.Dir(evidencePath)
		evidenceBase := strings.TrimSuffix(path.Base(evidencePath), path.Ext(evidencePath))
		if companionDir == evidenceDir || companionBase == evidenceBase {
			return true
		}
	}

	return false
}

func isAUTH001TestPath(filePath string) bool {
	return isConventionalTestPath(filePath)
}

func isAUTH001SecurityNotePath(filePath string) bool {
	if isGeneratedPath(filePath) || isExamplePath(filePath) || isConventionalTestPath(filePath) {
		return false
	}
	if isDocCompanionPath(filePath) {
		return true
	}

	base := strings.ToLower(path.Base(filePath))
	return base == "security.md"
}

func fileHasAUTH001SecurityNote(file model.File, context auth001CompanionContext) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != model.LineKindAdded || strings.TrimSpace(line.Text) == "" {
				continue
			}
			if !matchesAUTH001CompanionContext(file, line, context) {
				continue
			}
			if isAUTH001SecurityNoteLine(line.Text) {
				return true
			}
		}
	}
	return false
}

func isAUTH001AllowLine(text string) bool {
	return strings.Contains(strings.ToLower(text), "status ok") || hasAUTH001LineMarker(text, auth001AllowMarkers)
}

func isAUTH001DenyLine(text string) bool {
	return hasAUTH001LineMarker(text, auth001DenyMarkers)
}

func hasAUTH001LineMarker(text string, markers map[string]struct{}) bool {
	lower := strings.ToLower(text)
	for _, token := range termTokenRE.FindAllString(lower, -1) {
		cleaned := strings.Trim(token, "\"'`:,()[]{}<>")
		if hasAUTH001MarkerToken(cleaned, markers) {
			return true
		}
		for _, part := range strings.FieldsFunc(cleaned, func(r rune) bool {
			return r == '/' || r == '_' || r == '-' || r == '.'
		}) {
			if hasAUTH001MarkerToken(part, markers) {
				return true
			}
		}
	}

	return false
}

func hasAUTH001MarkerToken(token string, markers map[string]struct{}) bool {
	if token == "" {
		return false
	}
	_, ok := markers[token]
	return ok
}

var auth001AllowMarkers = map[string]struct{}{
	"allow":     {},
	"allowed":   {},
	"allows":    {},
	"grant":     {},
	"granted":   {},
	"grants":    {},
	"permit":    {},
	"permits":   {},
	"permitted": {},
	"statusok":  {},
	"success":   {},
	"succeeded": {},
	"succeeds":  {},
}

var auth001DenyMarkers = map[string]struct{}{
	"401":                {},
	"403":                {},
	"denied":             {},
	"denies":             {},
	"deny":               {},
	"disallow":           {},
	"disallowed":         {},
	"disallows":          {},
	"forbid":             {},
	"forbidden":          {},
	"forbids":            {},
	"reject":             {},
	"rejected":           {},
	"rejects":            {},
	"statusforbidden":    {},
	"statusunauthorized": {},
	"unauthenticated":    {},
	"unauthorized":       {},
}

func isAUTH001SecurityNoteLine(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "security") ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "authz") ||
		strings.Contains(lower, "permission")
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

func isERR001SuppressedPath(filePath string) bool {
	return isERR001CompanionPath(filePath) || isGeneratedPath(filePath) || isExamplePath(filePath)
}

func isOPS001TriggerPath(filePath string) bool {
	if isGeneratedPath(filePath) || isDocCompanionPath(filePath) || isConventionalTestPath(filePath) || isExamplePath(filePath) {
		return false
	}

	lower := strings.ToLower(filePath)
	for _, segment := range []string{
		"worker",
		"workers",
		"queue",
		"queues",
		"topic",
		"topics",
		"messaging",
		"pubsub",
		"kafka",
		"sqs",
		"sns",
		"rabbitmq",
		"consumer",
		"consumers",
		"subscriber",
		"subscribers",
		"publisher",
		"publishers",
		"job",
		"jobs",
		"task",
		"tasks",
		"cron",
		"scheduler",
		"schedulers",
		"retry",
		"retries",
		"backoff",
	} {
		if hasPathSegment(lower, segment) {
			return true
		}
	}

	base := strings.ToLower(path.Base(filePath))
	baseName := strings.TrimSuffix(base, path.Ext(base))
	if isOPS001TopicBaseName(baseName) {
		return true
	}

	for _, hint := range []string{"worker", "queue", "consumer", "subscriber", "publisher", "job", "cron", "schedule", "retry", "backoff"} {
		if strings.Contains(base, hint) {
			return true
		}
	}
	return false
}

func matchesOPS001Line(filePath, text string) bool {
	if isCommentLike(text) {
		return false
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	if isOPS001SyntaxOnlyLine(lower) {
		return false
	}

	for _, trigger := range []string{
		"queue",
		"topic",
		"consumer",
		"consume",
		"subscriber",
		"subscribe",
		"publisher",
		"publish",
		".ack(",
		".nack(",
		"deadletter",
		"dead_letter",
		"dead-letter",
		"dlq",
		"retry",
		"retries",
		"retryable",
		"maxattempt",
		"max_attempt",
		"maxinflight",
		"max_in_flight",
		"max-in-flight",
		"prefetch",
		"concurrency",
		"parallelism",
		"batchsize",
		"batch_size",
		"batch-size",
		"timeout",
		"deadline",
		"withtimeout",
		"cron",
		"schedule",
		"scheduler",
		"@every",
		"backoff",
		"jitter",
		"interval",
		"poll",
	} {
		if strings.Contains(lower, trigger) {
			return true
		}
	}

	if isOPS001CronPath(filePath) && cronExpressionRE.MatchString(lower) {
		return true
	}

	return isOPS001TimingPath(filePath) && (strings.Contains(lower, "time.") || durationLiteralRE.MatchString(lower))
}

func isOPS001TopicBaseName(baseName string) bool {
	return baseName == "topic" ||
		baseName == "topics" ||
		strings.Contains(baseName, "_topic") ||
		strings.Contains(baseName, "topic_") ||
		strings.Contains(baseName, "-topic") ||
		strings.Contains(baseName, "topic-")
}

func isOPS001SyntaxOnlyLine(lower string) bool {
	return strings.HasPrefix(lower, "package ") ||
		strings.HasPrefix(lower, "import ") ||
		lower == "import (" ||
		isOPS001ImportPathLine(lower)
}

func isOPS001ImportPathLine(lower string) bool {
	return strings.HasPrefix(lower, `"`) && strings.HasSuffix(lower, `"`) && strings.Contains(lower, "/")
}

func isOPS001CronPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	if hasPathSegment(lower, "cron") || hasPathSegment(lower, "scheduler") || hasPathSegment(lower, "schedulers") {
		return true
	}

	base := strings.ToLower(path.Base(filePath))
	return strings.Contains(base, "cron") || strings.Contains(base, "schedule")
}

func isOPS001TimingPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	for _, segment := range []string{"worker", "workers", "queue", "queues", "consumer", "consumers", "retry", "retries", "backoff", "cron", "scheduler", "schedulers"} {
		if hasPathSegment(lower, segment) {
			return true
		}
	}
	return false
}

type ops001CompanionContext struct {
	terms             []string
	fallbackPathTerms []string
}

func hasOPS001Companion(diff model.Diff, context ops001CompanionContext) bool {
	return len(collectOPS001CompanionPaths(diff, context)) > 0
}

func collectOPS001CompanionPaths(diff model.Diff, context ops001CompanionContext) []string {
	paths := make([]string, 0, 2)
	for _, file := range diff.Files {
		if file.Status == model.FileStatusDeleted || !isOPS001CompanionPath(file.Path) {
			continue
		}
		if isMetadataOnlyCompanionMove(file) && isOPS001CompanionPathRelated(file.Path, context) {
			paths = appendUniquePath(paths, file.Path)
			continue
		}
		if fileHasOPS001Companion(file, context) {
			paths = appendUniquePath(paths, file.Path)
		}
	}
	return paths
}

func buildOPS001CompanionContext(evidence []model.Evidence) ops001CompanionContext {
	return ops001CompanionContext{
		terms:             extractOPS001SearchTerms(evidence),
		fallbackPathTerms: extractOPS001FallbackPathTerms(evidence),
	}
}

func extractOPS001SearchTerms(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, 8)

	for _, evidenceItem := range evidence {
		for _, source := range []string{evidenceItem.File, evidenceItem.Excerpt} {
			for _, token := range termTokenRE.FindAllString(source, -1) {
				addOPS001SearchTerms(seen, &terms, token)
			}
		}
	}

	return terms
}

func addOPS001SearchTerms(seen map[string]struct{}, terms *[]string, token string) {
	cleaned := strings.ToLower(strings.Trim(token, "\"'`:,()[]{}<>"))
	if cleaned == "" {
		return
	}

	addOPS001SearchTerm(seen, terms, cleaned)
	for _, part := range strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.'
	}) {
		addOPS001SearchTerm(seen, terms, part)
	}
	if strings.HasSuffix(cleaned, "s") && !strings.HasSuffix(cleaned, "ss") {
		addOPS001SearchTerm(seen, terms, strings.TrimSuffix(cleaned, "s"))
	}
}

func addOPS001SearchTerm(seen map[string]struct{}, terms *[]string, term string) {
	if len(term) < 3 {
		return
	}
	if _, blocked := ignoredOPS001SearchTerms[term]; blocked {
		return
	}
	if _, exists := seen[term]; exists {
		return
	}

	seen[term] = struct{}{}
	*terms = append(*terms, term)
}

func extractOPS001FallbackPathTerms(evidence []model.Evidence) []string {
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(evidence))

	for _, evidenceItem := range evidence {
		base := strings.ToLower(path.Base(evidenceItem.File))
		if ext := path.Ext(base); ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
		addOPS001FallbackPathTerm(seen, &terms, base)
		for _, segment := range strings.Split(strings.ToLower(path.Dir(evidenceItem.File)), "/") {
			addOPS001FallbackPathTerm(seen, &terms, segment)
		}
	}

	return terms
}

func addOPS001FallbackPathTerm(seen map[string]struct{}, terms *[]string, term string) {
	if len(term) < 3 {
		return
	}
	if _, blocked := ignoredOPS001FallbackPathTerms[term]; blocked {
		return
	}
	if _, exists := seen[term]; exists {
		return
	}

	seen[term] = struct{}{}
	*terms = append(*terms, term)
}

var ignoredOPS001SearchTerms = map[string]struct{}{
	"ack":           {},
	"backoff":       {},
	"consume":       {},
	"consumer":      {},
	"consumers":     {},
	"cron":          {},
	"deadline":      {},
	"event":         {},
	"events":        {},
	"go":            {},
	"internal":      {},
	"kafka":         {},
	"interval":      {},
	"job":           {},
	"jobs":          {},
	"jitter":        {},
	"max":           {},
	"maxattempt":    {},
	"maxattempts":   {},
	"max_attempt":   {},
	"maxinflight":   {},
	"max_in_flight": {},
	"message":       {},
	"messages":      {},
	"messaging":     {},
	"ms":            {},
	"nack":          {},
	"operations":    {},
	"ops":           {},
	"poll":          {},
	"pubsub":        {},
	"publish":       {},
	"publisher":     {},
	"publishers":    {},
	"queue":         {},
	"queues":        {},
	"rabbitmq":      {},
	"retries":       {},
	"retry":         {},
	"retryable":     {},
	"schedule":      {},
	"scheduler":     {},
	"schedulers":    {},
	"second":        {},
	"seconds":       {},
	"service":       {},
	"sns":           {},
	"sqs":           {},
	"subscribe":     {},
	"subscriber":    {},
	"subscribers":   {},
	"task":          {},
	"tasks":         {},
	"time":          {},
	"timeout":       {},
	"topics":        {},
	"topic":         {},
	"ts":            {},
	"worker":        {},
	"workers":       {},
}

var ignoredOPS001FallbackPathTerms = map[string]struct{}{
	"go":         {},
	"internal":   {},
	"kafka":      {},
	"message":    {},
	"messages":   {},
	"messaging":  {},
	"operations": {},
	"ops":        {},
	"pubsub":     {},
	"rabbitmq":   {},
	"service":    {},
	"sns":        {},
	"sqs":        {},
}

func isOPS001CompanionPath(filePath string) bool {
	if isGeneratedPath(filePath) || isExamplePath(filePath) || isConventionalTestPath(filePath) {
		return false
	}
	return isDocCompanionPath(filePath) || isOPS001StrongCompanionPath(filePath)
}

func isOPS001StrongCompanionPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	for _, segment := range []string{
		"alerts",
		"dashboards",
		"grafana",
		"monitoring",
		"monitors",
		"observability",
		"oncall",
		"playbook",
		"playbooks",
		"prometheus",
		"rollback",
		"rollbacks",
		"runbook",
		"runbooks",
		"slo",
	} {
		if hasPathSegment(lower, segment) {
			return true
		}
	}
	return false
}

func isOPS001CompanionPathRelated(filePath string, context ops001CompanionContext) bool {
	return matchesOPS001CompanionContext(strings.ToLower(filePath), "", context)
}

func fileHasOPS001Companion(file model.File, context ops001CompanionContext) bool {
	lowerPath := strings.ToLower(file.Path)
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != model.LineKindAdded || strings.TrimSpace(line.Text) == "" {
				continue
			}
			if !isDocCompanionPath(file.Path) && isCommentLike(line.Text) {
				continue
			}
			if !matchesOPS001CompanionContext(lowerPath, strings.ToLower(line.Text), context) {
				continue
			}
			if isOPS001CompanionLine(line.Text) {
				return true
			}
		}
	}
	return false
}

func matchesOPS001CompanionContext(lowerPath, lowerText string, context ops001CompanionContext) bool {
	if len(context.terms) == 0 && len(context.fallbackPathTerms) == 0 {
		return true
	}

	normalizedPath := normalizeOPS001ContextText(lowerPath)
	normalizedText := normalizeOPS001ContextText(lowerText)

	for _, term := range context.terms {
		if containsOPS001ContextTerm(lowerPath, normalizedPath, term) || containsOPS001ContextTerm(lowerText, normalizedText, term) {
			return true
		}
	}
	for _, term := range context.fallbackPathTerms {
		if containsOPS001ContextTerm(lowerPath, normalizedPath, term) || containsOPS001ContextTerm(lowerText, normalizedText, term) {
			return true
		}
	}
	return false
}

func containsOPS001ContextTerm(raw, normalized, term string) bool {
	if strings.Contains(raw, term) {
		return true
	}

	normalizedTerm := normalizeOPS001ContextText(term)
	return normalizedTerm != "" && strings.Contains(normalized, normalizedTerm)
}

func normalizeOPS001ContextText(text string) string {
	return strings.Join(alnumTokenRE.FindAllString(strings.ToLower(text), -1), "")
}

func isOPS001CompanionLine(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"alert",
		"dashboard",
		"metric",
		"monitor",
		"observability",
		"on-call",
		"oncall",
		"operator",
		"pager",
		"playbook",
		"rollback",
		"roll back",
		"runbook",
		"trace",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isDOC001Path(filePath string) bool {
	if isDocCompanionPath(filePath) || isConventionalTestPath(filePath) || isExamplePath(filePath) {
		return false
	}

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

func isGeneratedPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := strings.ToLower(path.Base(filePath))

	return strings.HasSuffix(base, ".pb.go") ||
		strings.Contains(base, ".generated.") ||
		strings.HasPrefix(base, "generated_") ||
		hasPathSegment(lower, "generated") ||
		hasPathSegment(lower, "gen")
}

func isExamplePath(filePath string) bool {
	lower := strings.ToLower(filePath)
	return hasPathSegment(lower, "examples") ||
		hasPathSegment(lower, "example") ||
		hasTopLevelPathSegment(lower, "samples") ||
		hasTopLevelPathSegment(lower, "sample")
}

func hasTopLevelPathSegment(filePath, segment string) bool {
	return filePath == segment || strings.HasPrefix(filePath, segment+"/")
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
		strings.HasPrefix(lower, "testdata/") ||
		strings.Contains(lower, "/testdata/") ||
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

func collectPositiveCompanionPaths(diff model.Diff, pathMatch func(string) bool, terms []string, lineMatch func(model.File, model.Line, []string) bool) []string {
	paths := make([]string, 0, 2)
	for _, file := range diff.Files {
		if !pathMatch(file.Path) || file.Status == model.FileStatusDeleted {
			continue
		}
		if isMetadataOnlyCompanionMove(file) {
			paths = appendUniquePath(paths, file.Path)
			continue
		}
		if fileHasPositiveChange(file, terms, lineMatch) {
			paths = appendUniquePath(paths, file.Path)
		}
	}
	return paths
}

func appendUniquePath(paths []string, filePath string) []string {
	for _, existing := range paths {
		if existing == filePath {
			return paths
		}
	}
	return append(paths, filePath)
}

func isMetadataOnlyCompanionMove(file model.File) bool {
	if len(file.Hunks) != 0 {
		return false
	}

	switch file.Status {
	case model.FileStatusAdded, model.FileStatusModified, model.FileStatusRenamed, model.FileStatusCopied:
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
