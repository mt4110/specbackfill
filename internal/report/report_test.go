package report

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mt4110/specbackfill/internal/model"
	"github.com/mt4110/specbackfill/internal/rules"
)

func TestWriteTextSkeleton(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{
		Files: []model.File{{Path: "dir/file.txt", Status: model.FileStatusModified}},
	}
	result := Build(model.RepoProfile{Go: true}, nil)

	if err := Write(&output, "text", diff, result); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "specbackfill check") {
		t.Fatalf("text output missing command header:\n%s", text)
	}
	if !strings.Contains(text, "changed files: 1") {
		t.Fatalf("text output missing changed file count:\n%s", text)
	}
	if !strings.Contains(text, "changed file summary:") || !strings.Contains(text, "- other: 1") {
		t.Fatalf("text output missing changed file summary:\n%s", text)
	}
	if !strings.Contains(text, "    - dir/file.txt") {
		t.Fatalf("text output missing changed file sample:\n%s", text)
	}
	if !strings.Contains(text, "No findings emitted.") {
		t.Fatalf("text output missing empty findings message:\n%s", text)
	}
}

func TestTodoCountLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		count        int
		wantText     string
		wantMarkdown string
	}{
		{
			count:        0,
			wantText:     "0 unresolved obligations",
			wantMarkdown: "- Unresolved obligations: 0",
		},
		{
			count:        1,
			wantText:     "1 unresolved obligation",
			wantMarkdown: "- Unresolved obligation: 1",
		},
		{
			count:        2,
			wantText:     "2 unresolved obligations",
			wantMarkdown: "- Unresolved obligations: 2",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantText, func(t *testing.T) {
			t.Parallel()

			if got := todoCountText(tc.count); got != tc.wantText {
				t.Fatalf("todoCountText(%d) = %q, want %q", tc.count, got, tc.wantText)
			}
			if got := todoCountMarkdown(tc.count); got != tc.wantMarkdown {
				t.Fatalf("todoCountMarkdown(%d) = %q, want %q", tc.count, got, tc.wantMarkdown)
			}
		})
	}
}

func TestFileSummaryRows(t *testing.T) {
	t.Parallel()

	diff := model.Diff{Files: []model.File{
		{Path: "schema.prisma"},
		{Path: "prisma/migrations/20260512010101_add_user/migration.sql"},
		{Path: "openapi.yaml"},
		{Path: "generated/openapi/client.gen.ts"},
		{Path: "docs/usage.md"},
		{Path: "internal/api/handler_test.go"},
		{Path: "testdata/golden/text/db001_positive.golden"},
		{Path: ".github/workflows/ci.yaml"},
		{Path: "scripts/check.sh"},
		{Path: "internal/server/main.go"},
		{Path: "internal/server/router.go"},
		{Path: "internal/server/store.go"},
		{Path: "internal/server/worker.go"},
		{Path: "crates/domain/src/lib.rs"},
		{Path: "web/src/app.tsx"},
		{Path: "web/src/app.js"},
		{Path: "tools/load.py"},
		{Path: "queries/report.sql"},
		{Path: "examples/config.go"},
		{Path: "assets/logo.png"},
	}}

	rows := fileSummaryRows(diff)
	got := map[string]int{}
	byLabel := map[string]fileSummaryRow{}
	for _, row := range rows {
		got[row.label] = row.count
		byLabel[row.label] = row
	}

	for _, want := range []fileSummaryRow{
		{label: "db schema", count: 1},
		{label: "migrations", count: 1},
		{label: "API specs", count: 1},
		{label: "generated", count: 1},
		{label: "docs", count: 1},
		{label: "tests", count: 1},
		{label: "test fixtures", count: 1},
		{label: "config/ci", count: 1},
		{label: "scripts", count: 1},
		{label: "Go source", count: 4},
		{label: "Rust source", count: 1},
		{label: "TypeScript source", count: 1},
		{label: "JavaScript source", count: 1},
		{label: "Python source", count: 1},
		{label: "SQL", count: 1},
		{label: "examples/samples", count: 1},
		{label: "other", count: 1},
	} {
		if got[want.label] != want.count {
			t.Fatalf("summary %q = %d, want %d; rows=%+v", want.label, got[want.label], want.count, rows)
		}
	}

	goSource := byLabel["Go source"]
	if len(goSource.samples) != maxFileSummarySamples {
		t.Fatalf("Go source samples = %v, want %d samples", goSource.samples, maxFileSummarySamples)
	}
	if strings.Join(goSource.samples, ",") != "internal/server/main.go,internal/server/router.go,internal/server/store.go" {
		t.Fatalf("Go source samples = %v, want first changed files", goSource.samples)
	}
}

func TestWriteJSONSkeleton(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	result := Build(model.RepoProfile{Go: true}, nil)

	if err := Write(&output, "json", model.Diff{}, result); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["version"] != "v0" {
		t.Fatalf("version = %#v, want %q", decoded["version"], "v0")
	}
	if _, ok := decoded["summary"].(map[string]any); !ok {
		t.Fatalf("summary missing or invalid: %#v", decoded["summary"])
	}
	if findings, ok := decoded["findings"].([]any); !ok || len(findings) != 0 {
		t.Fatalf("findings = %#v, want empty array", decoded["findings"])
	}
}

func TestWriteMarkdownFinding(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{Go: true}, []model.Finding{{
		RuleID:     "DB001",
		Severity:   model.SeverityError,
		Confidence: "high",
		Title:      "Schema changed, but no matching migration companion moved with this diff",
		Why:        "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		ExpectedCompanions: []string{"migration file"},
	}})

	if err := Write(&output, "markdown", diff, result); err != nil {
		t.Fatalf("Write(markdown) error = %v", err)
	}

	markdown := output.String()
	for _, want := range []string{
		"### specbackfill findings",
		"#### [error] DB001 Schema changed, but no matching migration companion moved with this diff",
		"`schema.prisma`: `+ email String @unique`",
		"- migration file",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown output missing %q:\n%s", want, markdown)
		}
	}
}

func TestWriteMarkdownEscapesBacktickEvidence(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "internal/config/load.go", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{}, []model.Finding{{
		RuleID:     "CFG001",
		Severity:   model.SeverityWarn,
		Confidence: "high",
		Title:      "New config detected, but no matching docs/default companion moved with this diff",
		Why:        "A new config/env/flag line moved in the diff, but no matching docs/default companion evidence moved with it.",
		Evidence: []model.Evidence{{
			File:    "internal/config/load.go",
			Kind:    string(model.LineKindAdded),
			Excerpt: "token := os.Getenv(`NEW_API_TOKEN`)",
		}},
		ExpectedCompanions: []string{"docs"},
	}})

	if err := Write(&output, "markdown", diff, result); err != nil {
		t.Fatalf("Write(markdown) error = %v", err)
	}

	want := "``+ token := os.Getenv(`NEW_API_TOKEN`)``"
	if !strings.Contains(output.String(), want) {
		t.Fatalf("markdown output missing escaped backtick evidence %q:\n%s", want, output.String())
	}
}

func TestWriteTextSummary(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{}, []model.Finding{
		{RuleID: "DB001", Severity: model.SeverityError, Evidence: []model.Evidence{{File: "schema.prisma"}}, ExpectedCompanions: []string{"migration file"}},
		{RuleID: "API001", Severity: model.SeverityWarn, Evidence: []model.Evidence{{File: "openapi.yaml"}}, ExpectedCompanions: []string{"contract test"}},
		{RuleID: "API001", Severity: model.SeverityWarn, Evidence: []model.Evidence{{File: "openapi.yaml"}}, ExpectedCompanions: []string{"contract test"}},
	})

	err := WriteWithOptions(&output, "text", diff, result, Options{SummaryOnly: true})
	if err != nil {
		t.Fatalf("WriteWithOptions(summary text) error = %v", err)
	}

	text := output.String()
	for _, want := range []string{
		"specbackfill summary",
		"changed files: 1",
		"error: 1",
		"warn:  2",
		"info:  0",
		"- DB001: 1",
		"- API001: 2",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "expected companions:") {
		t.Fatalf("summary output included finding details:\n%s", text)
	}
}

func TestWriteJSONSummary(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	diff := model.Diff{Files: []model.File{{Path: "schema.prisma", Status: model.FileStatusModified}}}
	result := Build(model.RepoProfile{Go: true}, []model.Finding{{
		RuleID:             "DB001",
		Severity:           model.SeverityError,
		Evidence:           []model.Evidence{{File: "schema.prisma"}},
		ExpectedCompanions: []string{"migration file"},
	}})

	err := WriteWithOptions(&output, "json", diff, result, Options{SummaryOnly: true})
	if err != nil {
		t.Fatalf("WriteWithOptions(summary json) error = %v", err)
	}

	var decoded struct {
		Version      string         `json:"version"`
		ChangedFiles int            `json:"changed_files"`
		Summary      model.Summary  `json:"summary"`
		RulesFired   map[string]int `json:"rules_fired"`
		Findings     []any          `json:"findings"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\n%s", err, output.String())
	}
	if decoded.Version != "v0" || decoded.ChangedFiles != 1 || decoded.Summary.Error != 1 || decoded.RulesFired["DB001"] != 1 {
		t.Fatalf("decoded summary = %+v, want DB001 error summary", decoded)
	}
	if decoded.Findings != nil {
		t.Fatalf("summary JSON included findings: %+v", decoded.Findings)
	}
}

func TestBuildAddsStableFindingID(t *testing.T) {
	t.Parallel()

	findings := []model.Finding{{
		RuleID:     "DB001",
		Severity:   model.SeverityError,
		Confidence: "high",
		Title:      "Schema changed",
		Why:        "schema evidence moved",
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Line:    3,
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		ExpectedCompanions: []string{"migration file"},
	}}

	first := Build(model.RepoProfile{}, findings)
	second := Build(model.RepoProfile{}, findings)

	const wantID = "v0-8362254e793872c2"
	if first.Findings[0].FindingID != wantID {
		t.Fatalf("FindingID = %q, want %q", first.Findings[0].FindingID, wantID)
	}
	if first.Findings[0].FindingID != second.Findings[0].FindingID {
		t.Fatalf("FindingID not stable: first=%q second=%q", first.Findings[0].FindingID, second.Findings[0].FindingID)
	}
	if first.Findings[0].OmissionSignature != "db001.schema_changed.migration_companion" {
		t.Fatalf("OmissionSignature = %q, want DB001 signature", first.Findings[0].OmissionSignature)
	}
	if findings[0].FindingID != "" {
		t.Fatalf("Build mutated input finding: %+v", findings[0])
	}
	if findings[0].OmissionSignature != "" {
		t.Fatalf("Build mutated input omission signature: %+v", findings[0])
	}
}

func TestBuildObligationArtifactAddsVersionedMetadataAndIDs(t *testing.T) {
	t.Parallel()

	baseObligation := model.Obligation{
		RuleID:         "DB001",
		RuleVersion:    "v0",
		Status:         model.ObligationStatusMissing,
		Severity:       model.SeverityError,
		Confidence:     "high",
		Title:          "Schema changed, but no matching migration companion moved with this diff",
		Why:            "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		DiffLocalClaim: true,
		Anchor: model.ObligationAnchor{
			Kind: "schema_change",
			Path: "schema.prisma",
			Line: intPtr(3),
			Evidence: []model.Evidence{{
				File:    "schema.prisma",
				Line:    3,
				Kind:    string(model.LineKindAdded),
				Excerpt: "email String @unique",
			}},
		},
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          "migration_companion",
			Status:        model.ObligationStatusMissing,
			Satisfiers:    []string{},
			ExpectedPaths: []string{"prisma/migrations/**"},
		}},
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Line:    3,
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		Downstream: model.DownstreamMetadata{
			ImportKind:   "deterministic_static_layer",
			SourceSignal: "specbackfill",
		},
		ExpectedCompanions: []string{"migration file"},
	}

	missing := BuildObligationArtifact(ObligationArtifactOptions{
		InputKind: "diff_file",
		DiffInput: []byte("diff"),
	}, []model.Obligation{baseObligation})

	if missing.SchemaVersion != "obligations.v1" || missing.Tool.Name != "specbackfill" || missing.Run.InputKind != "diff_file" {
		t.Fatalf("artifact metadata = %+v, want obligations.v1 specbackfill diff_file", missing)
	}
	if missing.Run.Base != nil || missing.Run.Head != nil {
		t.Fatalf("run refs = base %v head %v, want nil refs", missing.Run.Base, missing.Run.Head)
	}
	if missing.Run.RunID == "" || !strings.HasPrefix(missing.Run.DiffFingerprint, "sha256:") {
		t.Fatalf("run IDs missing: %+v", missing.Run)
	}
	if len(missing.Obligations) != 1 {
		t.Fatalf("len(obligations) = %d, want 1", len(missing.Obligations))
	}
	missingObligation := missing.Obligations[0]
	if missingObligation.ObligationID == "" || missingObligation.FindingID == nil || missingObligation.OmissionSignature == nil {
		t.Fatalf("missing obligation IDs not populated: %+v", missingObligation)
	}

	satisfiedObligation := baseObligation
	satisfiedObligation.Status = model.ObligationStatusSatisfied
	satisfiedObligation.RequiredCompanions[0].Status = model.ObligationStatusSatisfied
	satisfiedObligation.RequiredCompanions[0].Satisfiers = []string{"prisma/migrations/20260329010101_add_email/migration.sql"}
	satisfiedObligation.RequiredCompanions[0].SatisfierEvidence = []model.Evidence{{
		File:    "prisma/migrations/20260329010101_add_email/migration.sql",
		Line:    1,
		Kind:    string(model.LineKindAdded),
		Excerpt: "ALTER TABLE \"User\" ADD COLUMN \"email\" TEXT;",
	}}
	satisfied := BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{satisfiedObligation})
	if satisfied.Obligations[0].ObligationID != missingObligation.ObligationID {
		t.Fatalf("obligation ID changed across status: missing=%q satisfied=%q", missingObligation.ObligationID, satisfied.Obligations[0].ObligationID)
	}
	if satisfied.Obligations[0].FindingID != nil || satisfied.Obligations[0].OmissionSignature != nil {
		t.Fatalf("satisfied obligation has finding metadata: %+v", satisfied.Obligations[0])
	}
	if satisfied.Obligations[0].StatusReason == nil || satisfied.Obligations[0].StatusReason.Reason != model.StatusReasonCompanionPresent {
		t.Fatalf("satisfied obligation status reason = %+v, want companion_present", satisfied.Obligations[0].StatusReason)
	}

	malformed := baseObligation
	malformed.RequiredCompanions = nil
	artifact := BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{malformed})
	if artifact.Obligations[0].RequiredCompanions == nil {
		t.Fatalf("nil required companions were not normalized: %+v", artifact.Obligations[0])
	}
}

func TestBuildLocalAIReviewImportItems(t *testing.T) {
	t.Parallel()

	baseObligation := model.Obligation{
		RuleID:         "DB001",
		RuleVersion:    "v0",
		Status:         model.ObligationStatusMissing,
		Severity:       model.SeverityError,
		Confidence:     "high",
		Title:          "Schema changed, but no matching migration companion moved with this diff",
		Why:            "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		DiffLocalClaim: true,
		Anchor: model.ObligationAnchor{
			Kind: "schema_change",
			Path: "schema.prisma",
			Line: intPtr(3),
			Evidence: []model.Evidence{{
				File:    "schema.prisma",
				Line:    3,
				Kind:    string(model.LineKindAdded),
				Excerpt: "email String @unique",
			}},
		},
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          "migration_companion",
			Status:        model.ObligationStatusMissing,
			Satisfiers:    []string{},
			ExpectedPaths: []string{"prisma/migrations/**"},
		}},
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Line:    3,
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		Downstream: model.DownstreamMetadata{
			ImportKind:   "deterministic_static_layer",
			SourceSignal: "specbackfill",
		},
		ExpectedCompanions: []string{"migration file"},
	}
	artifact := BuildObligationArtifact(ObligationArtifactOptions{
		InputKind: "diff_file",
		DiffInput: []byte("diff"),
	}, []model.Obligation{baseObligation})
	artifact.Tool.Name = "unexpected-tool-name"
	artifact.Obligations[0].Downstream = model.DownstreamMetadata{
		ImportKind:   "unexpected_import_kind",
		SourceSignal: "unexpected_signal",
	}

	items := BuildLocalAIReviewImportItems(artifact)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	item := items[0]
	obligation := artifact.Obligations[0]
	if item.SchemaVersion != "local_ai_review_import.v1" || item.Source != "specbackfill" || item.ImportKind != "deterministic_static_layer" {
		t.Fatalf("import metadata = %+v, want local_ai_review_import.v1 specbackfill deterministic_static_layer", item)
	}
	if item.RunID != artifact.Run.RunID || item.DiffFingerprint != artifact.Run.DiffFingerprint {
		t.Fatalf("run metadata = %+v, want artifact run %+v", item, artifact.Run)
	}
	if item.ItemID != obligation.ObligationID || item.ObligationID != obligation.ObligationID {
		t.Fatalf("item IDs = item_id %q obligation_id %q, want %q", item.ItemID, item.ObligationID, obligation.ObligationID)
	}
	if item.FindingID == nil || item.OmissionSignature == nil {
		t.Fatalf("missing import item did not preserve finding metadata: %+v", item)
	}
	if item.EvidenceDigest == "" || !strings.HasPrefix(item.EvidenceDigest, "sha256:") || len(item.EvidenceDigest) != len("sha256:")+64 {
		t.Fatalf("EvidenceDigest = %q, want sha256 digest", item.EvidenceDigest)
	}
	if !item.DiffLocalClaim || len(item.Evidence) == 0 || len(item.Anchor.Evidence) == 0 || len(item.RequiredCompanions) == 0 {
		t.Fatalf("import item lost diff-local evidence: %+v", item)
	}
	if len(item.RawJSON) == 0 {
		t.Fatalf("import item missing raw obligation JSON: %+v", item)
	}
	var rawObligation model.Obligation
	if err := json.Unmarshal(item.RawJSON, &rawObligation); err != nil {
		t.Fatalf("raw_json is not an obligation object: %v\n%s", err, string(item.RawJSON))
	}
	if rawObligation.ObligationID != item.ObligationID || rawObligation.RuleID != item.RuleID {
		t.Fatalf("raw_json does not preserve source obligation: raw=%+v item=%+v", rawObligation, item)
	}
}

func TestBuildLocalAIReviewImportItemsPreservesDecodedArtifactIDs(t *testing.T) {
	t.Parallel()

	baseObligation := model.Obligation{
		RuleID:         "DB001",
		RuleVersion:    "v0",
		Status:         model.ObligationStatusMissing,
		Severity:       model.SeverityError,
		Confidence:     "high",
		Title:          "Schema changed, but no matching migration companion moved with this diff",
		Why:            "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
		DiffLocalClaim: true,
		Anchor: model.ObligationAnchor{
			Kind: "schema_change",
			Path: "schema.prisma",
			Line: intPtr(3),
			Evidence: []model.Evidence{{
				File:    "schema.prisma",
				Line:    3,
				Kind:    string(model.LineKindAdded),
				Excerpt: "email String @unique",
			}},
		},
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          "migration_companion",
			Status:        model.ObligationStatusMissing,
			ExpectedPaths: []string{"prisma/migrations/**"},
		}},
		Evidence: []model.Evidence{{
			File:    "schema.prisma",
			Line:    3,
			Kind:    string(model.LineKindAdded),
			Excerpt: "email String @unique",
		}},
		ExpectedCompanions: []string{"migration file"},
	}

	artifact := BuildObligationArtifact(ObligationArtifactOptions{
		InputKind: "diff_file",
		DiffInput: []byte("diff"),
	}, []model.Obligation{baseObligation})
	wantObligation := artifact.Obligations[0]
	if wantObligation.FindingID == nil || wantObligation.OmissionSignature == nil {
		t.Fatalf("test setup missing generated finding metadata: %+v", wantObligation)
	}

	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("json.Marshal(artifact) error = %v", err)
	}

	var decoded model.ObligationArtifact
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(artifact) error = %v", err)
	}

	items := BuildLocalAIReviewImportItems(decoded)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.ObligationID != wantObligation.ObligationID || item.ItemID != wantObligation.ObligationID {
		t.Fatalf("decoded artifact IDs changed: item=%+v want obligation ID %q", item, wantObligation.ObligationID)
	}
	if item.FindingID == nil || *item.FindingID != *wantObligation.FindingID {
		t.Fatalf("decoded artifact finding ID = %v, want %v", item.FindingID, wantObligation.FindingID)
	}
	if item.OmissionSignature == nil || *item.OmissionSignature != *wantObligation.OmissionSignature {
		t.Fatalf("decoded artifact omission signature = %v, want %v", item.OmissionSignature, wantObligation.OmissionSignature)
	}
}

func TestLocalAIReviewEvidenceDigestChangesWithCompanionEvidence(t *testing.T) {
	t.Parallel()

	obligation := model.Obligation{
		RuleID:         "DB001",
		RuleVersion:    "v0",
		Status:         model.ObligationStatusMissing,
		Severity:       model.SeverityError,
		Confidence:     "high",
		Title:          "Schema changed",
		Why:            "schema evidence moved",
		DiffLocalClaim: true,
		Anchor: model.ObligationAnchor{
			Kind:     "schema_change",
			Path:     "schema.prisma",
			Evidence: []model.Evidence{{File: "schema.prisma", Line: 3, Kind: string(model.LineKindAdded), Excerpt: "email String @unique"}},
		},
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          "migration_companion",
			Status:        model.ObligationStatusMissing,
			ExpectedPaths: []string{"prisma/migrations/**"},
		}},
		Evidence:           []model.Evidence{{File: "schema.prisma", Line: 3, Kind: string(model.LineKindAdded), Excerpt: "email String @unique"}},
		ExpectedCompanions: []string{"migration file"},
	}

	missingArtifact := BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{obligation})
	missingItem := BuildLocalAIReviewImportItems(missingArtifact)[0]

	satisfied := obligation
	satisfied.Status = model.ObligationStatusSatisfied
	satisfied.RequiredCompanions[0].Status = model.ObligationStatusSatisfied
	satisfied.RequiredCompanions[0].Satisfiers = []string{"prisma/migrations/20260329010101_add_email/migration.sql"}
	satisfied.RequiredCompanions[0].SatisfierEvidence = []model.Evidence{{
		File:    "prisma/migrations/20260329010101_add_email/migration.sql",
		Line:    1,
		Kind:    string(model.LineKindAdded),
		Excerpt: "ALTER TABLE \"User\" ADD COLUMN \"email\" TEXT;",
	}}
	satisfiedArtifact := BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{satisfied})
	satisfiedItem := BuildLocalAIReviewImportItems(satisfiedArtifact)[0]

	if missingItem.ItemID != satisfiedItem.ItemID {
		t.Fatalf("ItemID changed across status: missing=%q satisfied=%q", missingItem.ItemID, satisfiedItem.ItemID)
	}
	if missingItem.EvidenceDigest == satisfiedItem.EvidenceDigest {
		t.Fatalf("EvidenceDigest did not change when companion evidence changed: %q", missingItem.EvidenceDigest)
	}
	if satisfiedItem.FindingID != nil || satisfiedItem.OmissionSignature != nil {
		t.Fatalf("satisfied import item has finding metadata: %+v", satisfiedItem)
	}
}

func TestLocalAIReviewEvidenceDigestChangesWithStatusReasons(t *testing.T) {
	t.Parallel()

	obligation := model.Obligation{
		RuleID:         "CFG001",
		RuleVersion:    "v0",
		Status:         model.ObligationStatusSuppressed,
		Severity:       model.SeverityWarn,
		Confidence:     "medium",
		Title:          "CFG001 evidence matched a documented negative condition",
		Why:            "suppressed",
		DiffLocalClaim: true,
		Anchor: model.ObligationAnchor{
			Kind:     "suppressed_config_reference",
			Path:     "docs/config.md",
			Evidence: []model.Evidence{{File: "docs/config.md", Line: 1, Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("NEW_TOKEN")`}},
		},
		RequiredCompanions: []model.RequiredCompanion{{
			Kind:          "config_docs_default_companion",
			Status:        model.ObligationStatusSuppressed,
			ExpectedPaths: []string{"docs/**"},
		}},
		Evidence: []model.Evidence{{File: "docs/config.md", Line: 1, Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("NEW_TOKEN")`}},
		Suppression: &model.ObligationSuppression{
			Reason:   model.SuppressionReasonDocsOnly,
			Evidence: []model.Evidence{{File: "docs/config.md", Line: 1, Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("NEW_TOKEN")`}},
		},
	}
	docsItem := BuildLocalAIReviewImportItems(BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{obligation}))[0]

	testsOnly := obligation
	testsOnly.Suppression = &model.ObligationSuppression{
		Reason:   model.SuppressionReasonTestsOnly,
		Evidence: obligation.Suppression.Evidence,
	}
	testsItem := BuildLocalAIReviewImportItems(BuildObligationArtifact(ObligationArtifactOptions{InputKind: "diff_file", DiffInput: []byte("diff")}, []model.Obligation{testsOnly}))[0]

	if docsItem.EvidenceDigest == testsItem.EvidenceDigest {
		t.Fatalf("EvidenceDigest did not change when suppression reason changed: %q", docsItem.EvidenceDigest)
	}
}

func TestLocalAIReviewImportSchemaDocumentsRequiredFields(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../schemas/local_ai_review_import.schema.json")
	if err != nil {
		t.Fatalf("ReadFile(schema) error = %v", err)
	}

	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Defs       map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("json.Unmarshal(schema) error = %v", err)
	}

	for _, field := range []string{
		"schema_version",
		"source",
		"import_kind",
		"run_id",
		"item_id",
		"rule_id",
		"status",
		"severity",
		"title",
		"diff_local_claim",
		"evidence_digest",
	} {
		if !containsString(schema.Required, field) {
			t.Fatalf("schema required fields missing %q: %v", field, schema.Required)
		}
		if _, ok := schema.Properties[field]; !ok {
			t.Fatalf("schema properties missing %q", field)
		}
	}
	for _, field := range []string{"status_reason", "raw_json"} {
		if containsString(schema.Required, field) {
			t.Fatalf("schema must keep additive v1 field %q optional for backward compatibility: %v", field, schema.Required)
		}
		if _, ok := schema.Properties[field]; !ok {
			t.Fatalf("schema properties missing additive producer field %q", field)
		}
	}

	var rawJSONProperty struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(schema.Properties["raw_json"], &rawJSONProperty); err != nil {
		t.Fatalf("json.Unmarshal(raw_json property) error = %v", err)
	}
	if rawJSONProperty.Ref != "#/$defs/obligation" {
		t.Fatalf("raw_json schema ref = %q, want #/$defs/obligation", rawJSONProperty.Ref)
	}

	var obligationDef struct {
		Required             []string       `json:"required"`
		Properties           map[string]any `json:"properties"`
		AdditionalProperties *bool          `json:"additionalProperties"`
	}
	if err := json.Unmarshal(schema.Defs["obligation"], &obligationDef); err != nil {
		t.Fatalf("json.Unmarshal(obligation def) error = %v", err)
	}
	for _, field := range []string{"finding_id", "omission_signature", "anchor", "required_companions", "downstream"} {
		if !containsString(obligationDef.Required, field) {
			t.Fatalf("raw_json obligation schema missing required field %q: %v", field, obligationDef.Required)
		}
		if _, ok := obligationDef.Properties[field]; !ok {
			t.Fatalf("raw_json obligation schema properties missing %q", field)
		}
	}
	if obligationDef.AdditionalProperties == nil || *obligationDef.AdditionalProperties {
		t.Fatalf("raw_json obligation schema must close additional properties")
	}
}

func TestLocalAIReviewImportGoldensContainSchemaRequiredFields(t *testing.T) {
	t.Parallel()

	required := localAIReviewImportSchemaRequiredFields(t)
	entries, err := os.ReadDir("../../testdata/golden/local_ai_review_import")
	if err != nil {
		t.Fatalf("ReadDir(goldens) error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("local-ai-review import goldens are empty")
	}

	for _, entry := range entries {
		entry := entry
		if entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile("../../testdata/golden/local_ai_review_import/" + entry.Name())
			if err != nil {
				t.Fatalf("ReadFile(golden) error = %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			for _, line := range lines {
				var decoded map[string]any
				if err := json.Unmarshal([]byte(line), &decoded); err != nil {
					t.Fatalf("json.Unmarshal(golden line) error = %v\nline=%s", err, line)
				}
				for _, field := range required {
					if _, ok := decoded[field]; !ok {
						t.Fatalf("golden %s missing required field %q\nline=%s", entry.Name(), field, line)
					}
				}
				for _, field := range []string{"status_reason", "raw_json"} {
					if _, ok := decoded[field]; !ok {
						t.Fatalf("golden %s missing emitted producer field %q\nline=%s", entry.Name(), field, line)
					}
				}

				var item model.LocalAIReviewImportItem
				if err := json.Unmarshal([]byte(line), &item); err != nil {
					t.Fatalf("json.Unmarshal(item) error = %v\nline=%s", err, line)
				}
				if item.SchemaVersion != "local_ai_review_import.v1" || item.Source != "specbackfill" || item.ImportKind != "deterministic_static_layer" || item.SourceSignal != "specbackfill" {
					t.Fatalf("golden %s has invalid adapter identity: %+v", entry.Name(), item)
				}
				if !strings.HasPrefix(item.EvidenceDigest, "sha256:") || len(item.EvidenceDigest) != len("sha256:")+64 {
					t.Fatalf("golden %s evidence digest = %q, want sha256 digest", entry.Name(), item.EvidenceDigest)
				}
				if item.Status == model.ObligationStatusSatisfied {
					if item.StatusReason == nil || item.StatusReason.Reason != model.StatusReasonCompanionPresent || len(item.StatusReason.Evidence) == 0 {
						t.Fatalf("golden %s satisfied item missing companion_present reason: %+v", entry.Name(), item)
					}
					if len(item.RequiredCompanions) == 0 || len(item.RequiredCompanions[0].Satisfiers) == 0 || len(item.RequiredCompanions[0].SatisfierEvidence) == 0 {
						t.Fatalf("golden %s satisfied item missing satisfier evidence: %+v", entry.Name(), item)
					}
				}
				if item.Status == model.ObligationStatusSuppressed {
					if item.StatusReason == nil || len(item.StatusReason.Evidence) == 0 {
						t.Fatalf("golden %s suppressed item missing status reason evidence: %+v", entry.Name(), item)
					}
					if item.Suppression == nil || len(item.Suppression.Evidence) == 0 {
						t.Fatalf("golden %s suppressed item missing suppression evidence: %+v", entry.Name(), item)
					}
				}
			}
		})
	}
}

func TestBuildOverwritesGeneratedFindingMetadata(t *testing.T) {
	t.Parallel()

	result := Build(model.RepoProfile{}, []model.Finding{{
		FindingID:         "non-deterministic-input-id",
		OmissionSignature: "non-deterministic-input-signature",
		RuleID:            "CFG001",
		Severity:          model.SeverityWarn,
		Confidence:        "medium",
		Title:             "New config detected",
		Why:               "config evidence moved",
		Evidence:          []model.Evidence{{File: "config.go", Kind: string(model.LineKindAdded), Excerpt: `os.Getenv("FOO")`}},
		ExpectedCompanions: []string{
			"docs",
		},
	}})

	if got := result.Findings[0].FindingID; got == "" || got == "non-deterministic-input-id" {
		t.Fatalf("FindingID = %q, want generated deterministic ID", got)
	}
	if got := result.Findings[0].OmissionSignature; got != "cfg001.config_introduced.docs_default" {
		t.Fatalf("OmissionSignature = %q, want generated CFG001 signature", got)
	}
}

func intPtr(value int) *int {
	return &value
}

func localAIReviewImportSchemaRequiredFields(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile("../../schemas/local_ai_review_import.schema.json")
	if err != nil {
		t.Fatalf("ReadFile(schema) error = %v", err)
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("json.Unmarshal(schema) error = %v", err)
	}
	if len(schema.Required) == 0 {
		t.Fatalf("schema has no required fields")
	}
	return schema.Required
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func TestBuildAddsOmissionSignatureForEveryCatalogRule(t *testing.T) {
	t.Parallel()

	for _, info := range rules.Catalog() {
		info := info
		t.Run(info.ID, func(t *testing.T) {
			t.Parallel()

			result := Build(model.RepoProfile{}, []model.Finding{{
				RuleID:             info.ID,
				Severity:           info.DefaultSeverity,
				Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
				ExpectedCompanions: []string{"companion"},
			}})

			signature := result.Findings[0].OmissionSignature
			if signature == "" {
				t.Fatalf("%s OmissionSignature is empty", info.ID)
			}
			if strings.HasSuffix(signature, ".unmapped") {
				t.Fatalf("%s OmissionSignature = %q, want explicit mapped signature", info.ID, signature)
			}
		})
	}
}

func TestBuildAddsDeterministicOmissionSignatureFallback(t *testing.T) {
	t.Parallel()

	result := Build(model.RepoProfile{}, []model.Finding{{
		RuleID:             "NEW999",
		Severity:           model.SeverityInfo,
		Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
		ExpectedCompanions: []string{"companion"},
	}})

	if got := result.Findings[0].OmissionSignature; got != "new999.unmapped" {
		t.Fatalf("OmissionSignature = %q, want deterministic fallback", got)
	}

	emptyRuleResult := Build(model.RepoProfile{}, []model.Finding{{
		Severity:           model.SeverityInfo,
		Evidence:           []model.Evidence{{File: "file.go", Kind: string(model.LineKindAdded), Excerpt: "changed line"}},
		ExpectedCompanions: []string{"companion"},
	}})
	if got := emptyRuleResult.Findings[0].OmissionSignature; got != "unknown.rule_id.unmapped" {
		t.Fatalf("empty-rule OmissionSignature = %q, want unknown fallback", got)
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	findings := []model.Finding{
		{Severity: model.SeverityWarn},
		{Severity: model.SeverityError},
	}

	if code := ExitCode(findings, "error"); code != 1 {
		t.Fatalf("ExitCode(error) = %d, want 1", code)
	}
	if code := ExitCode(findings[:1], "error"); code != 0 {
		t.Fatalf("ExitCode(error without error finding) = %d, want 0", code)
	}
	if code := ExitCode(findings[:1], "warn"); code != 1 {
		t.Fatalf("ExitCode(warn) = %d, want 1", code)
	}
	if code := ExitCode(findings, "off"); code != 0 {
		t.Fatalf("ExitCode(off) = %d, want 0", code)
	}
}
