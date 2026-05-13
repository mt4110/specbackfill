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
		{name: "db001 metadata-only rename negative", file: "db001_metadata_rename.diff", want: nil},
		{name: "db002 positive", file: "db002_positive.diff", want: []string{"DB002"}},
		{name: "db002 companion satisfied", file: "db002_companion.diff", want: nil},
		{name: "db002 removed-only companion satisfied", file: "db002_removed_companion.diff", want: nil},
		{name: "db002 paraphrased companion satisfied", file: "db002_paraphrased_companion.diff", want: nil},
		{name: "db002 deleted companion", file: "db002_deleted_companion.diff", want: []string{"DB002"}},
		{name: "db002 unrelated companion", file: "db002_unrelated_companion.diff", want: []string{"DB002"}},
		{name: "db002 additive migration negative", file: "db002_negative_additive.diff", want: nil},
		{name: "db002 ambiguous negative", file: "db002_negative_ambiguous.diff", want: nil},
		{name: "db002 metadata-only migration rename negative", file: "db002_metadata_migration_rename.diff", want: nil},
		{name: "db001 and db002 composite", file: "db001_db002_positive.diff", want: []string{"DB001", "DB002"}},
		{name: "cfg001 positive", file: "cfg001_positive.diff", want: []string{"CFG001"}},
		{name: "cfg001 companion satisfied", file: "cfg001_companion.diff", want: nil},
		{name: "cfg001 deleted docs companion", file: "cfg001_deleted_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 docs only", file: "cfg001_docs_only.diff", want: nil},
		{name: "cfg001 positive with api001 suppressed", file: "cfg001_positive_api001_suppressed.diff", want: []string{"CFG001"}},
		{name: "cfg001 removed unrelated docs still warns", file: "cfg001_removed_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 unrelated docs companion", file: "cfg001_unrelated_docs.diff", want: []string{"CFG001"}},
		{name: "cfg001 comment only", file: "cfg001_comment_only.diff", want: nil},
		{name: "cfg001 generated only negative", file: "cfg001_negative_generated_only.diff", want: nil},
		{name: "cfg001 tests only negative", file: "cfg001_negative_tests_only.diff", want: nil},
		{name: "cfg001 examples only negative", file: "cfg001_negative_examples_only.diff", want: nil},
		{name: "cfg001 nested examples only negative", file: "cfg001_negative_nested_examples_only.diff", want: nil},
		{name: "cfg001 samples only negative", file: "cfg001_negative_samples_only.diff", want: nil},
		{name: "cfg001 sample named production positive", file: "cfg001_sample_named_production_positive.diff", want: []string{"CFG001"}},
		{name: "cfg001 sample dir production positive", file: "cfg001_sample_dir_production_positive.diff", want: []string{"CFG001"}},
		{name: "cfg001 metadata-only rename negative", file: "cfg001_metadata_rename.diff", want: nil},
		{name: "api001 positive", file: "api001_positive.diff", want: []string{"API001"}},
		{name: "api001 and err001 composite", file: "api001_err001_positive.diff", want: []string{"API001", "ERR001"}},
		{name: "api001 companion satisfied", file: "api001_companion.diff", want: nil},
		{name: "api001 deleted docs companion", file: "api001_deleted_docs.diff", want: []string{"API001"}},
		{name: "api001 docs only", file: "api001_docs_only.diff", want: nil},
		{name: "api001 unrelated docs companion", file: "api001_unrelated_docs.diff", want: []string{"API001"}},
		{name: "api001 generated openapi routes to doc001 only", file: "api001_generated_openapi_routes_to_doc001.diff", want: []string{"DOC001"}},
		{name: "api001 tests only negative", file: "api001_negative_tests_only.diff", want: nil},
		{name: "api001 examples only negative", file: "api001_negative_examples_only.diff", want: nil},
		{name: "api001 samples only negative", file: "api001_negative_samples_only.diff", want: nil},
		{name: "api001 metadata-only rename negative", file: "api001_metadata_rename.diff", want: nil},
		{name: "auth001 positive", file: "auth001_positive.diff", want: []string{"AUTH001"}},
		{name: "auth001 middleware positive", file: "auth001_middleware_positive.diff", want: []string{"AUTH001"}},
		{name: "auth001 allow deny companion satisfied", file: "auth001_companion.diff", want: nil},
		{name: "auth001 allow deny companion empty context satisfied", file: "auth001_companion_empty_context.diff", want: nil},
		{name: "auth001 allow deny companion without specific terms satisfied", file: "auth001_companion_no_specific_terms.diff", want: nil},
		{name: "auth001 security note empty context satisfied", file: "auth001_security_note_empty_context.diff", want: nil},
		{name: "auth001 metadata test rename satisfied", file: "auth001_metadata_test_rename.diff", want: nil},
		{name: "auth001 security note companion satisfied", file: "auth001_security_note_companion.diff", want: nil},
		{name: "auth001 deleted companion", file: "auth001_deleted_companion.diff", want: []string{"AUTH001"}},
		{name: "auth001 empty context unrelated tests still warns", file: "auth001_empty_context_unrelated_tests.diff", want: []string{"AUTH001"}},
		{name: "auth001 removed companion", file: "auth001_removed_companion.diff", want: []string{"AUTH001"}},
		{name: "auth001 unrelated companion", file: "auth001_unrelated_companion.diff", want: []string{"AUTH001"}},
		{name: "auth001 security code unrelated companion", file: "auth001_security_code_unrelated.diff", want: []string{"AUTH001"}},
		{name: "auth001 unrelated 200 with deny still warns", file: "auth001_unrelated_200_with_deny.diff", want: []string{"AUTH001"}},
		{name: "auth001 examples only negative", file: "auth001_negative_examples_only.diff", want: nil},
		{name: "auth001 samples only negative", file: "auth001_negative_samples_only.diff", want: nil},
		{name: "auth001 testdata golden negative", file: "auth001_negative_testdata_golden.diff", want: nil},
		{name: "auth001 non-auth path negative", file: "auth001_negative_non_auth_path.diff", want: nil},
		{name: "auth001 generated only negative", file: "auth001_negative_generated_only.diff", want: nil},
		{name: "auth001 tests only negative", file: "auth001_negative_tests_only.diff", want: nil},
		{name: "auth001 metadata-only rename negative", file: "auth001_metadata_rename.diff", want: nil},
		{name: "err001 positive", file: "err001_positive.diff", want: []string{"ERR001"}},
		{name: "err001 companion satisfied", file: "err001_companion.diff", want: nil},
		{name: "err001 removed-only companion satisfied", file: "err001_removed_companion.diff", want: nil},
		{name: "err001 paraphrased companion satisfied", file: "err001_paraphrased_companion.diff", want: nil},
		{name: "err001 deleted companion", file: "err001_deleted_companion.diff", want: []string{"ERR001"}},
		{name: "err001 unrelated companion", file: "err001_unrelated_companion.diff", want: []string{"ERR001"}},
		{name: "err001 message only negative", file: "err001_negative_message_only.diff", want: nil},
		{name: "err001 comment only negative", file: "err001_negative_comment_only.diff", want: nil},
		{name: "err001 generated only negative", file: "err001_negative_generated_only.diff", want: nil},
		{name: "err001 tests only negative", file: "err001_negative_tests_only.diff", want: nil},
		{name: "err001 examples only negative", file: "err001_negative_examples_only.diff", want: nil},
		{name: "err001 samples only negative", file: "err001_negative_samples_only.diff", want: nil},
		{name: "err001 metadata-only rename negative", file: "err001_metadata_rename.diff", want: nil},
		{name: "ops001 positive", file: "ops001_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 ops path positive", file: "ops001_ops_path_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 topic positive", file: "ops001_topic_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 topic basename positive", file: "ops001_topic_basename_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 cron positive", file: "ops001_cron_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 cron named fields positive", file: "ops001_cron_named_fields_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 cron midnight positive", file: "ops001_cron_midnight_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 consumer behavior positive", file: "ops001_consumer_behavior_positive.diff", want: []string{"OPS001"}},
		{name: "ops001 companion satisfied", file: "ops001_companion.diff", want: nil},
		{name: "ops001 fallback companion satisfied", file: "ops001_fallback_companion.diff", want: nil},
		{name: "ops001 observability companion satisfied", file: "ops001_observability_companion.diff", want: nil},
		{name: "ops001 deleted companion", file: "ops001_deleted_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 removed companion", file: "ops001_removed_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 unrelated companion", file: "ops001_unrelated_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 unrelated observability companion", file: "ops001_unrelated_observability_companion.diff", want: []string{"OPS001"}},
		{name: "ops001 non ops path negative", file: "ops001_negative_non_ops_path.diff", want: nil},
		{name: "ops001 generated only negative", file: "ops001_negative_generated_only.diff", want: nil},
		{name: "ops001 docs only negative", file: "ops001_negative_docs_only.diff", want: nil},
		{name: "ops001 tests only negative", file: "ops001_negative_tests_only.diff", want: nil},
		{name: "ops001 examples only negative", file: "ops001_negative_examples_only.diff", want: nil},
		{name: "ops001 samples only negative", file: "ops001_negative_samples_only.diff", want: nil},
		{name: "ops001 metadata-only rename negative", file: "ops001_metadata_rename.diff", want: nil},
		{name: "doc001 positive", file: "doc001_positive.diff", want: []string{"DOC001"}},
		{name: "doc001 companion satisfied", file: "doc001_companion.diff", want: nil},
		{name: "doc001 deleted docs companion", file: "doc001_deleted_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 ambiguous", file: "doc001_ambiguous.diff", want: nil},
		{name: "doc001 unrelated docs companion", file: "doc001_unrelated_docs.diff", want: []string{"DOC001"}},
		{name: "doc001 docs only", file: "api001_docs_only.diff", want: nil},
		{name: "doc001 generated docs only negative", file: "doc001_negative_docs_only.diff", want: nil},
		{name: "doc001 generated tests only negative", file: "doc001_negative_tests_only.diff", want: nil},
		{name: "doc001 generated examples only negative", file: "doc001_negative_examples_only.diff", want: nil},
		{name: "doc001 generated samples only negative", file: "doc001_negative_samples_only.diff", want: nil},
		{name: "doc001 metadata-only rename negative", file: "doc001_metadata_rename.diff", want: nil},
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

func TestEvaluateObligationsStatuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		file       string
		wantRule   string
		wantStatus model.ObligationStatus
		wantCount  int
	}{
		{
			name:       "positive missing obligation",
			file:       "db001_positive.diff",
			wantRule:   "DB001",
			wantStatus: model.ObligationStatusMissing,
			wantCount:  1,
		},
		{
			name:       "companion-present satisfied obligation",
			file:       "db001_companion.diff",
			wantRule:   "DB001",
			wantStatus: model.ObligationStatusSatisfied,
			wantCount:  1,
		},
		{
			name:       "negative migration-only emits suppressed obligation",
			file:       "db001_migration_only.diff",
			wantRule:   "DB001",
			wantStatus: model.ObligationStatusSuppressed,
			wantCount:  1,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obligations := EvaluateObligations(parseFixture(t, tc.file), model.RepoProfile{})
			if len(obligations) != tc.wantCount {
				t.Fatalf("len(obligations) = %d, want %d: %+v", len(obligations), tc.wantCount, obligations)
			}
			if tc.wantCount == 0 {
				return
			}

			obligation := obligations[0]
			if obligation.RuleID != tc.wantRule || obligation.Status != tc.wantStatus {
				t.Fatalf("obligation = %+v, want rule %s status %s", obligation, tc.wantRule, tc.wantStatus)
			}
			if !obligation.DiffLocalClaim {
				t.Fatalf("DiffLocalClaim = false, want true")
			}
			if obligation.Anchor.Path == "" || len(obligation.Anchor.Evidence) == 0 || len(obligation.Evidence) == 0 {
				t.Fatalf("obligation missing anchor evidence: %+v", obligation)
			}
			if len(obligation.RequiredCompanions) == 0 {
				t.Fatalf("obligation missing required companions: %+v", obligation)
			}
			if obligation.RequiredCompanions[0].Status != tc.wantStatus {
				t.Fatalf("required companion status = %s, want %s", obligation.RequiredCompanions[0].Status, tc.wantStatus)
			}
			if tc.wantStatus == model.ObligationStatusSatisfied && len(obligation.RequiredCompanions[0].Satisfiers) == 0 {
				t.Fatalf("satisfied obligation missing satisfiers: %+v", obligation)
			}
			if tc.wantStatus == model.ObligationStatusSuppressed && obligation.Suppression == nil {
				t.Fatalf("suppressed obligation missing suppression reason: %+v", obligation)
			}
		})
	}
}

func TestSatisfiedObligationSatisfiersAreSpecificCompanionPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		file      string
		wantRule  string
		wantPaths []string
	}{
		{
			name:     "auth allow deny",
			file:     "auth001_companion.diff",
			wantRule: "AUTH001",
			wantPaths: []string{
				"internal/auth/permissions_test.go",
			},
		},
		{
			name:     "ops runbook",
			file:     "ops001_companion.diff",
			wantRule: "OPS001",
			wantPaths: []string{
				"docs/runbooks/billing-worker.md",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obligations := EvaluateObligations(parseFixture(t, tc.file), model.RepoProfile{})
			if len(obligations) != 1 {
				t.Fatalf("len(obligations) = %d, want 1: %+v", len(obligations), obligations)
			}

			obligation := obligations[0]
			if obligation.RuleID != tc.wantRule || obligation.Status != model.ObligationStatusSatisfied {
				t.Fatalf("obligation = %+v, want %s satisfied", obligation, tc.wantRule)
			}
			gotPaths := obligation.RequiredCompanions[0].Satisfiers
			if !reflect.DeepEqual(gotPaths, tc.wantPaths) {
				t.Fatalf("satisfiers = %v, want %v", gotPaths, tc.wantPaths)
			}
		})
	}
}

func TestSatisfiedObligationIncludesCompanionEvidenceAndReason(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		file             string
		wantRule         string
		wantEvidenceFile string
	}{
		{name: "db", file: "db001_companion.diff", wantRule: "DB001", wantEvidenceFile: "prisma/migrations/20260329010101_add_email/migration.sql"},
		{name: "api", file: "api001_companion.diff", wantRule: "API001", wantEvidenceFile: "docs/api.md"},
		{name: "auth", file: "auth001_companion.diff", wantRule: "AUTH001", wantEvidenceFile: "internal/auth/permissions_test.go"},
		{name: "err", file: "err001_companion.diff", wantRule: "ERR001", wantEvidenceFile: "internal/http/handler_test.go"},
		{name: "ops", file: "ops001_companion.diff", wantRule: "OPS001", wantEvidenceFile: "docs/runbooks/billing-worker.md"},
		{name: "doc", file: "doc001_companion.diff", wantRule: "DOC001", wantEvidenceFile: "CHANGELOG.md"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obligations := EvaluateObligations(parseFixture(t, tc.file), model.RepoProfile{})
			if len(obligations) != 1 {
				t.Fatalf("len(obligations) = %d, want 1", len(obligations))
			}

			obligation := obligations[0]
			if obligation.RuleID != tc.wantRule || obligation.Status != model.ObligationStatusSatisfied {
				t.Fatalf("obligation = %+v, want %s satisfied", obligation, tc.wantRule)
			}
			if obligation.StatusReason == nil || obligation.StatusReason.Reason != model.StatusReasonCompanionPresent {
				t.Fatalf("StatusReason = %+v, want companion_present", obligation.StatusReason)
			}
			if len(obligation.RequiredCompanions) != 1 || len(obligation.RequiredCompanions[0].SatisfierEvidence) == 0 {
				t.Fatalf("satisfied obligation missing companion evidence: %+v", obligation)
			}
			if len(obligation.StatusReason.Evidence) == 0 {
				t.Fatalf("status reason missing companion evidence: %+v", obligation)
			}
			gotEvidence := obligation.RequiredCompanions[0].SatisfierEvidence[0]
			if gotEvidence.File != tc.wantEvidenceFile || gotEvidence.Kind != string(model.LineKindAdded) {
				t.Fatalf("satisfier evidence = %+v, want added evidence in %s", gotEvidence, tc.wantEvidenceFile)
			}
		})
	}
}

func TestSuppressedObligationsIncludeReasonAndEvidence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		file       string
		wantRule   string
		wantReason model.SuppressionReason
	}{
		{name: "generated only", file: "cfg001_negative_generated_only.diff", wantRule: "CFG001", wantReason: model.SuppressionReasonGeneratedOnly},
		{name: "docs only", file: "cfg001_docs_only.diff", wantRule: "CFG001", wantReason: model.SuppressionReasonDocsOnly},
		{name: "tests only", file: "cfg001_negative_tests_only.diff", wantRule: "CFG001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "example only", file: "cfg001_negative_examples_only.diff", wantRule: "CFG001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "sample only", file: "cfg001_negative_samples_only.diff", wantRule: "CFG001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "api tests only", file: "api001_negative_tests_only.diff", wantRule: "API001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "api example only", file: "api001_negative_examples_only.diff", wantRule: "API001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "api sample only", file: "api001_negative_samples_only.diff", wantRule: "API001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "auth generated only", file: "auth001_negative_generated_only.diff", wantRule: "AUTH001", wantReason: model.SuppressionReasonGeneratedOnly},
		{name: "auth tests only", file: "auth001_negative_tests_only.diff", wantRule: "AUTH001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "auth example only", file: "auth001_negative_examples_only.diff", wantRule: "AUTH001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "auth sample only", file: "auth001_negative_samples_only.diff", wantRule: "AUTH001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "err generated only", file: "err001_negative_generated_only.diff", wantRule: "ERR001", wantReason: model.SuppressionReasonGeneratedOnly},
		{name: "err tests only", file: "err001_negative_tests_only.diff", wantRule: "ERR001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "err example only", file: "err001_negative_examples_only.diff", wantRule: "ERR001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "err sample only", file: "err001_negative_samples_only.diff", wantRule: "ERR001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "ops generated only", file: "ops001_negative_generated_only.diff", wantRule: "OPS001", wantReason: model.SuppressionReasonGeneratedOnly},
		{name: "ops docs only", file: "ops001_negative_docs_only.diff", wantRule: "OPS001", wantReason: model.SuppressionReasonDocsOnly},
		{name: "ops tests only", file: "ops001_negative_tests_only.diff", wantRule: "OPS001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "ops example only", file: "ops001_negative_examples_only.diff", wantRule: "OPS001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "ops sample only", file: "ops001_negative_samples_only.diff", wantRule: "OPS001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "doc docs only", file: "doc001_negative_docs_only.diff", wantRule: "DOC001", wantReason: model.SuppressionReasonDocsOnly},
		{name: "doc tests only", file: "doc001_negative_tests_only.diff", wantRule: "DOC001", wantReason: model.SuppressionReasonTestsOnly},
		{name: "doc example only", file: "doc001_negative_examples_only.diff", wantRule: "DOC001", wantReason: model.SuppressionReasonExampleOnly},
		{name: "doc sample only", file: "doc001_negative_samples_only.diff", wantRule: "DOC001", wantReason: model.SuppressionReasonSampleOnly},
		{name: "migration only", file: "db001_migration_only.diff", wantRule: "DB001", wantReason: model.SuppressionReasonMigrationOnly},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obligations := EvaluateObligations(parseFixture(t, tc.file), model.RepoProfile{})
			if len(obligations) != 1 {
				t.Fatalf("len(obligations) = %d, want 1: %+v", len(obligations), obligations)
			}
			obligation := obligations[0]
			if obligation.RuleID != tc.wantRule || obligation.Status != model.ObligationStatusSuppressed {
				t.Fatalf("obligation = %+v, want %s suppressed", obligation, tc.wantRule)
			}
			if obligation.Suppression == nil || obligation.Suppression.Reason != tc.wantReason {
				t.Fatalf("Suppression = %+v, want reason %s", obligation.Suppression, tc.wantReason)
			}
			if obligation.StatusReason == nil || obligation.StatusReason.Reason != model.StatusReason(tc.wantReason) {
				t.Fatalf("StatusReason = %+v, want reason %s", obligation.StatusReason, tc.wantReason)
			}
			if len(obligation.Suppression.Evidence) == 0 || len(obligation.Anchor.Evidence) == 0 {
				t.Fatalf("suppressed obligation missing evidence: %+v", obligation)
			}
			if len(FindingsFromObligations(obligations)) != 0 {
				t.Fatalf("suppressed obligation turned into finding: %+v", obligations)
			}
		})
	}
}

func TestSatisfierEvidenceIsBounded(t *testing.T) {
	t.Parallel()

	lines := make([]model.Line, 0, maxSatisfierEvidencePerFile+4)
	for index := 0; index < maxSatisfierEvidencePerFile+4; index++ {
		lines = append(lines, model.Line{
			Kind:    model.LineKindAdded,
			Text:    "user companion evidence",
			NewLine: index + 1,
		})
	}
	file := model.File{
		Path: "docs/api.md",
		Hunks: []model.Hunk{{
			Lines: lines,
		}},
	}

	fileEvidence := collectFilePositiveChangeEvidence(file, nil, func(model.File, model.Line, []string) bool {
		return true
	})
	if len(fileEvidence) != maxSatisfierEvidencePerFile {
		t.Fatalf("len(file evidence) = %d, want cap %d", len(fileEvidence), maxSatisfierEvidencePerFile)
	}

	matches := make([]companionMatch, 0, maxSatisfierEvidencePerObligation+1)
	for index := 0; index < maxSatisfierEvidencePerObligation+1; index++ {
		matches = append(matches, companionMatch{
			path: "docs/api.md",
			evidence: []model.Evidence{{
				File:    "docs/api.md",
				Line:    index + 1,
				Kind:    string(model.LineKindAdded),
				Excerpt: "user companion evidence",
			}},
		})
	}

	obligationEvidence := companionMatchEvidence(matches)
	if len(obligationEvidence) != maxSatisfierEvidencePerObligation {
		t.Fatalf("len(obligation evidence) = %d, want cap %d", len(obligationEvidence), maxSatisfierEvidencePerObligation)
	}
}

func TestFindingsIncludeRequiredFields(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"db001_positive.diff",
		"db002_positive.diff",
		"cfg001_positive.diff",
		"api001_positive.diff",
		"auth001_positive.diff",
		"err001_positive.diff",
		"ops001_positive.diff",
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

func TestMissingObligationsIncludeRequiredFields(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"db001_positive.diff",
		"db002_positive.diff",
		"cfg001_positive.diff",
		"api001_positive.diff",
		"auth001_positive.diff",
		"err001_positive.diff",
		"ops001_positive.diff",
		"doc001_positive.diff",
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			obligations := EvaluateObligations(parseFixture(t, fixture), model.RepoProfile{})
			if len(obligations) != 1 {
				t.Fatalf("len(obligations) = %d, want 1", len(obligations))
			}

			obligation := obligations[0]
			if obligation.Status != model.ObligationStatusMissing {
				t.Fatalf("obligation.Status = %s, want missing", obligation.Status)
			}
			if obligation.RuleID == "" || obligation.Severity == "" || obligation.Confidence == "" || obligation.Title == "" || obligation.Why == "" {
				t.Fatalf("obligation has empty required scalar field: %+v", obligation)
			}
			if !obligation.DiffLocalClaim {
				t.Fatalf("DiffLocalClaim = false, want true")
			}
			if obligation.Anchor.Kind == "" || obligation.Anchor.Path == "" || len(obligation.Anchor.Evidence) == 0 {
				t.Fatalf("obligation missing anchor evidence: %+v", obligation)
			}
			if len(obligation.Evidence) == 0 || len(obligation.Evidence) > 3 {
				t.Fatalf("len(obligation.Evidence) = %d, want 1..3", len(obligation.Evidence))
			}
			if len(obligation.RequiredCompanions) == 0 {
				t.Fatalf("required companions missing: %+v", obligation)
			}
			if obligation.RequiredCompanions[0].Status != model.ObligationStatusMissing {
				t.Fatalf("required companion status = %s, want missing", obligation.RequiredCompanions[0].Status)
			}
		})
	}
}

func TestScanAnchorRuleIDs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		file string
		want []string
	}{
		{name: "positive DB001 anchor", file: "db001_positive.diff", want: []string{"DB001"}},
		{name: "companion satisfied still has DB001 anchor", file: "db001_companion.diff", want: []string{"DB001"}},
		{name: "migration-only has no schema anchor", file: "db001_migration_only.diff", want: nil},
		{name: "composite anchors", file: "api001_err001_positive.diff", want: []string{"API001", "ERR001"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ScanAnchorRuleIDs(parseFixture(t, tc.file))
			if tc.want == nil {
				tc.want = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ScanAnchorRuleIDs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPIPathMatchingIsCaseInsensitiveForPathSegments(t *testing.T) {
	t.Parallel()

	if !isAPIPath("api/OpenAPI/users.yaml") {
		t.Fatalf("isAPIPath() = false, want true for OpenAPI path segment")
	}
	if !isAPIPath("Proto/users.proto") {
		t.Fatalf("isAPIPath() = false, want true for proto path segment")
	}
}

func TestMetadataOnlyCompanionMoveSuppressesFinding(t *testing.T) {
	t.Parallel()

	diff := model.Diff{
		Files: []model.File{
			{
				Path:    "openapi.yaml",
				OldPath: "openapi.yaml",
				NewPath: "openapi.yaml",
				Status:  model.FileStatusModified,
				Hunks: []model.Hunk{{
					Lines: []model.Line{{
						Kind:    model.LineKindAdded,
						Text:    "  /users:",
						NewLine: 2,
					}},
				}},
			},
			{
				Path:    "docs/api-v2.md",
				OldPath: "docs/api.md",
				NewPath: "docs/api-v2.md",
				Status:  model.FileStatusRenamed,
			},
		},
	}

	if got := ruleIDs(Evaluate(diff, model.RepoProfile{})); len(got) != 0 {
		t.Fatalf("ruleIDs = %v, want no findings", got)
	}
}

func TestExamplePathTreatsSampleAsTopLevelOnly(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"sample/config.go", "samples/config.go", "cmd/tool/example/config.go", "cmd/tool/examples/config.go"} {
		if !isExamplePath(path) {
			t.Fatalf("isExamplePath(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"internal/sample/config.go", "cmd/tool/sample/config.go", "cmd/tool/samples/config.go", "internal/config/sample_loader.go", "internal/sampler/config.go"} {
		if isExamplePath(path) {
			t.Fatalf("isExamplePath(%q) = true, want false", path)
		}
	}
}

func TestMetadataOnlyModifiedCompanionSuppressesFinding(t *testing.T) {
	t.Parallel()

	diff := model.Diff{
		Files: []model.File{
			{
				Path:    "openapi.yaml",
				OldPath: "openapi.yaml",
				NewPath: "openapi.yaml",
				Status:  model.FileStatusModified,
				Hunks: []model.Hunk{{
					Lines: []model.Line{{
						Kind:    model.LineKindAdded,
						Text:    "  /users:",
						NewLine: 2,
					}},
				}},
			},
			{
				Path:    "docs/api.md",
				OldPath: "docs/api.md",
				NewPath: "docs/api.md",
				Status:  model.FileStatusModified,
			},
		},
	}

	if got := ruleIDs(Evaluate(diff, model.RepoProfile{})); len(got) != 0 {
		t.Fatalf("ruleIDs = %v, want no findings", got)
	}
}

func TestCommentLikeAllowsPointerDerefAndMarkdownCompanionLines(t *testing.T) {
	t.Parallel()

	if isCommentLike("*ptr = value") {
		t.Fatalf("isCommentLike() = true for pointer dereference")
	}
	if !isCommentLike("* block comment continuation") {
		t.Fatalf("isCommentLike() = false for block comment continuation")
	}

	file := model.File{
		Path:   "docs/api.md",
		Status: model.FileStatusModified,
		Hunks: []model.Hunk{{
			Lines: []model.Line{{
				Kind: model.LineKindAdded,
				Text: "* Document /users endpoint",
			}},
		}},
	}
	if !fileHasPositiveChange(file, []string{"users"}, companionTermsMatch) {
		t.Fatalf("fileHasPositiveChange() = false, want markdown companion line to count")
	}
}

func TestAUTH001AllowDenyMarkersAreTokenBased(t *testing.T) {
	t.Parallel()

	if !isAUTH001AllowLine(`t.Run("allows valid user", func(t *testing.T) {})`) {
		t.Fatalf("isAUTH001AllowLine() = false, want true for allows marker")
	}
	if !isAUTH001AllowLine(`assert.Equal(t, http.StatusOK, status)`) {
		t.Fatalf("isAUTH001AllowLine() = false, want true for StatusOK marker")
	}
	if isAUTH001AllowLine(`expectedRetryBudget := 200`) {
		t.Fatalf("isAUTH001AllowLine() = true, want false for bare 200 marker")
	}
	if isAUTH001AllowLine(`t.Run("disallows viewer", func(t *testing.T) {})`) {
		t.Fatalf("isAUTH001AllowLine() = true, want false for disallows marker")
	}
	if isAUTH001AllowLine(`t.Run("unsuccessful login", func(t *testing.T) {})`) {
		t.Fatalf("isAUTH001AllowLine() = true, want false for unsuccessful marker")
	}
	if !isAUTH001DenyLine(`t.Run("disallows viewer", func(t *testing.T) {})`) {
		t.Fatalf("isAUTH001DenyLine() = false, want true for disallows marker")
	}
	if !isAUTH001DenyLine(`assert.Equal(t, http.StatusForbidden, status)`) {
		t.Fatalf("isAUTH001DenyLine() = false, want true for StatusForbidden marker")
	}
}

func TestOPS001LineMatchingAvoidsPackageDeclarationFalsePositives(t *testing.T) {
	t.Parallel()

	if matchesOPS001Line("internal/workers/consumer.go", "package retry") {
		t.Fatalf("matchesOPS001Line() = true, want false for package declaration")
	}
	if matchesOPS001Line("internal/consumers/worker.go", "package consumers") {
		t.Fatalf("matchesOPS001Line() = true, want false for package declaration containing trigger term")
	}
	if matchesOPS001Line("internal/consumers/worker.go", `import "example.com/project/retry"`) {
		t.Fatalf("matchesOPS001Line() = true, want false for import declaration containing trigger term")
	}
	if matchesOPS001Line("internal/consumers/worker.go", `"example.com/project/retry"`) {
		t.Fatalf("matchesOPS001Line() = true, want false for import path containing trigger term")
	}
	if !matchesOPS001Line("internal/consumers/worker.go", "message.Ack(ctx)") {
		t.Fatalf("matchesOPS001Line() = false, want true for explicit Ack call")
	}
}

func TestOPS001CompanionContextNormalizesCompoundTerms(t *testing.T) {
	t.Parallel()

	context := ops001CompanionContext{terms: []string{"retrybackoff"}}
	if !matchesOPS001CompanionContext("docs/runbooks/billing-worker.md", "rollback path for retry backoff changes", context) {
		t.Fatalf("matchesOPS001CompanionContext() = false, want true for separated retry backoff wording")
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
