package rules

import (
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

type RuleInfo struct {
	ID                 string
	DefaultSeverity    model.Severity
	ListDescription    string
	Title              string
	TriggerBullets     []string
	ExpectedCompanions []string
	DoesNotReportOn    []string
}

var defaultRuleCatalog = []RuleInfo{
	{
		ID:              "DB001",
		DefaultSeverity: model.SeverityError,
		ListDescription: "Schema changed -> migration companion",
		Title:           "Schema changed, but no matching migration companion moved with this diff",
		TriggerBullets: []string{
			"schema.prisma changed",
			"db/schema.sql changed",
			"ent/schema/** or sqlc/schema/** changed",
			"CREATE TABLE / ALTER TABLE / ADD COLUMN / DROP COLUMN / CREATE INDEX",
			"Prisma field or index shape changed",
		},
		ExpectedCompanions: []string{
			"migration file",
			"migration test",
			"rollback/backfill note",
		},
		DoesNotReportOn: []string{
			"migration-only diffs with matching companion evidence",
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"companion artifacts moved with concrete companion evidence",
		},
	},
	{
		ID:              "DB002",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Destructive storage change -> rollback/backfill note",
		Title:           "Destructive storage change detected, but no matching rollback/backfill companion moved with this diff",
		TriggerBullets: []string{
			"DROP COLUMN / DROP TABLE",
			"SET NOT NULL",
			"CREATE UNIQUE INDEX",
			"ADD CONSTRAINT ... UNIQUE",
			"schema or migration file changed",
		},
		ExpectedCompanions: []string{
			"rollback note",
			"data backfill note",
			"compatibility test",
		},
		DoesNotReportOn: []string{
			"additive schema or migration diffs without a destructive signal",
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"rollback/backfill or compatibility companion evidence moved with this diff",
		},
	},
	{
		ID:              "CFG001",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Config/env introduced -> docs/default",
		Title:           "New config detected, but no matching docs/default companion moved with this diff",
		TriggerBullets: []string{
			"os.Getenv( or os.LookupEnv(",
			"process.env.",
			"viper.Get",
			"flag. or cobra. flag definition",
			"added config/env/flag line",
		},
		ExpectedCompanions: []string{
			"docs",
			"default value handling",
			"upgrade note",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"docs/default or upgrade companion evidence moved with this diff",
		},
	},
	{
		ID:              "API001",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Public API changed -> contract test/docs",
		Title:           "Public API changed, but no matching contract-test/docs companion moved with this diff",
		TriggerBullets: []string{
			"OpenAPI YAML changed",
			"schema.graphql changed",
			"proto/**/*.proto changed",
			"meaningful API spec line changed",
		},
		ExpectedCompanions: []string{
			"contract test",
			"API docs",
			"compatibility or deprecation note",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"contract-test/docs companion evidence moved with this diff",
		},
	},
	{
		ID:              "AUTH001",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Authz changed -> allow/deny tests",
		Title:           "Authn/Authz branch changed, but no matching allow/deny or security-sensitive note companion moved with this diff",
		TriggerBullets: []string{
			"auth/authz/permission path changed",
			"middleware, guard, or policy path changed",
			"authorize/permission/role/scope line changed",
			"forbidden/unauthorized branch changed",
		},
		ExpectedCompanions: []string{
			"allow test",
			"deny test",
			"security-sensitive note",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"matching allow/deny test evidence moved with this diff",
			"matching security-sensitive note evidence moved with this diff",
		},
	},
	{
		ID:              "ERR001",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Error contract changed -> assertion test",
		Title:           "Public error/status/code contract changed, but no matching assertion-test/docs companion moved with this diff",
		TriggerBullets: []string{
			"HTTP status code line changed",
			"gRPC code line changed",
			"public error/status/code contract line changed",
		},
		ExpectedCompanions: []string{
			"assertion test",
			"API or client note",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"assertion-test/docs companion evidence moved with this diff",
		},
	},
	{
		ID:              "OPS001",
		DefaultSeverity: model.SeverityWarn,
		ListDescription: "Worker/retry changed -> runbook/observability",
		Title:           "Worker/queue/retry behavior changed, but no matching observability/runbook companion moved with this diff",
		TriggerBullets: []string{
			"worker/queue/topic/consumer path changed",
			"retry/backoff/timeout behavior changed",
			"cron or scheduler behavior changed",
			"queue topic, concurrency, batch, or polling line changed",
		},
		ExpectedCompanions: []string{
			"observability note",
			"runbook update",
			"rollback path",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"generated-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"observability/runbook or rollback companion evidence moved with this diff",
		},
	},
	{
		ID:              "DOC001",
		DefaultSeverity: model.SeverityInfo,
		ListDescription: "Generated spec changed -> human explanation",
		Title:           "Generated spec/client changed, but no matching human-facing explanation moved with this diff",
		TriggerBullets: []string{
			"generated OpenAPI or Swagger client changed",
			"generated proto artifact changed",
			"generated GraphQL client/schema artifact changed",
			"meaningful generated spec/client line changed",
		},
		ExpectedCompanions: []string{
			"human docs",
			"upgrade note",
		},
		DoesNotReportOn: []string{
			"docs-only diffs",
			"tests-only diffs",
			"example-only or top-level sample-only diffs where no production contract moved",
			"human-facing explanation evidence moved with this diff",
		},
	},
}

var defaultRuleCatalogOrder = []string{"DB001", "DB002", "API001", "CFG001", "AUTH001", "ERR001", "OPS001", "DOC001"}

func Catalog() []RuleInfo {
	catalog := make([]RuleInfo, 0, len(defaultRuleCatalog))
	for _, ruleID := range defaultRuleCatalogOrder {
		info, ok := findRuleInfo(ruleID)
		if ok {
			catalog = append(catalog, cloneRuleInfo(info))
		}
	}
	return catalog
}

func FindRuleInfo(ruleID string) (RuleInfo, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(ruleID))
	info, ok := findRuleInfo(normalized)
	if !ok {
		return RuleInfo{}, false
	}
	return cloneRuleInfo(info), true
}

func findRuleInfo(ruleID string) (RuleInfo, bool) {
	for _, info := range defaultRuleCatalog {
		if info.ID == ruleID {
			return info, true
		}
	}
	return RuleInfo{}, false
}

func cloneRuleInfo(info RuleInfo) RuleInfo {
	info.TriggerBullets = cloneStrings(info.TriggerBullets)
	info.ExpectedCompanions = cloneStrings(info.ExpectedCompanions)
	info.DoesNotReportOn = cloneStrings(info.DoesNotReportOn)
	return info
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
