package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

type Options struct {
	SummaryOnly         bool
	InputSummary        string
	InputNotes          []string
	AnchorScanAvailable bool
	AnchorRuleIDs       []string
}

func Build(repoProfile model.RepoProfile, findings []model.Finding) model.Report {
	if findings == nil {
		findings = []model.Finding{}
	}
	findings = withFindingIDs(findings)

	return model.Report{
		Version:     "v0",
		Summary:     model.SummaryFromFindings(findings),
		RepoProfile: repoProfile,
		Findings:    findings,
	}
}

type ObligationArtifactOptions struct {
	ToolVersion string
	InputKind   string
	Base        string
	Head        string
	DiffInput   []byte
}

func BuildObligationArtifact(options ObligationArtifactOptions, obligations []model.Obligation) model.ObligationArtifact {
	if obligations == nil {
		obligations = []model.Obligation{}
	}

	diffFingerprint := diffFingerprint(options.DiffInput)
	runID := "run-" + strings.TrimPrefix(diffFingerprint, "sha256:")[:16]
	withIDs := withObligationIDs(obligations)

	return model.ObligationArtifact{
		SchemaVersion: "obligations.v1",
		Tool: model.ToolMetadata{
			Name:    "specbackfill",
			Version: toolVersion(options.ToolVersion),
		},
		Run: model.RunMetadata{
			RunID:           runID,
			InputKind:       options.InputKind,
			Base:            nullableString(options.Base),
			Head:            nullableString(options.Head),
			DiffFingerprint: diffFingerprint,
		},
		Obligations: withIDs,
	}
}

func toolVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "v0"
	}
	return trimmed
}

func BuildLocalAIReviewImportItems(artifact model.ObligationArtifact) []model.LocalAIReviewImportItem {
	obligations := normalizeObligations(artifact.Obligations, obligationNormalizationOptions{
		PreserveExistingIDs: true,
		ExpectedCompanions:  importExpectedCompanions,
	})
	items := make([]model.LocalAIReviewImportItem, 0, len(obligations))

	for _, obligation := range obligations {
		items = append(items, model.LocalAIReviewImportItem{
			SchemaVersion:      "local_ai_review_import.v1",
			Source:             "specbackfill",
			ImportKind:         "deterministic_static_layer",
			SourceSignal:       "specbackfill",
			ToolVersion:        artifact.Tool.Version,
			RunID:              artifact.Run.RunID,
			InputKind:          artifact.Run.InputKind,
			DiffFingerprint:    artifact.Run.DiffFingerprint,
			ItemID:             obligation.ObligationID,
			ObligationID:       obligation.ObligationID,
			FindingID:          obligation.FindingID,
			OmissionSignature:  obligation.OmissionSignature,
			RuleID:             obligation.RuleID,
			RuleVersion:        obligation.RuleVersion,
			Status:             obligation.Status,
			Severity:           obligation.Severity,
			Confidence:         obligation.Confidence,
			Title:              obligation.Title,
			Why:                obligation.Why,
			DiffLocalClaim:     obligation.DiffLocalClaim,
			EvidenceDigest:     stableEvidenceDigest(obligation),
			Anchor:             obligation.Anchor,
			RequiredCompanions: obligation.RequiredCompanions,
			Evidence:           obligation.Evidence,
			StatusReason:       obligation.StatusReason,
			Suppression:        obligation.Suppression,
			RawJSON:            rawObligationJSON(obligation),
		})
	}

	return items
}

func rawObligationJSON(obligation model.Obligation) json.RawMessage {
	raw, err := json.Marshal(obligation)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}

type obligationNormalizationOptions struct {
	PreserveExistingIDs bool
	ExpectedCompanions  func(model.Obligation) []string
}

func normalizeObligations(obligations []model.Obligation, options obligationNormalizationOptions) []model.Obligation {
	if obligations == nil {
		return []model.Obligation{}
	}

	normalized := make([]model.Obligation, len(obligations))
	copy(normalized, obligations)
	for index := range normalized {
		obligation := &normalized[index]
		normalizeObligationArtifactSlices(obligation)
		if !options.PreserveExistingIDs || obligation.ObligationID == "" {
			obligation.ObligationID = stableObligationID(*obligation)
		}
		if obligation.Status == model.ObligationStatusSatisfied && obligation.StatusReason == nil {
			satisfierEvidence := firstSatisfierEvidence(obligation.RequiredCompanions)
			if len(satisfierEvidence) > 0 {
				obligation.StatusReason = &model.ObligationStatusReason{
					Reason:   model.StatusReasonCompanionPresent,
					Evidence: satisfierEvidence,
				}
			}
		}
		if obligation.Status == model.ObligationStatusSuppressed && obligation.StatusReason == nil && obligation.Suppression != nil {
			obligation.StatusReason = &model.ObligationStatusReason{
				Reason:   model.StatusReason(obligation.Suppression.Reason),
				Evidence: obligation.Suppression.Evidence,
			}
		}
		if obligation.Status != model.ObligationStatusMissing {
			obligation.FindingID = nil
			obligation.OmissionSignature = nil
			continue
		}
		if !options.PreserveExistingIDs || obligation.FindingID == nil {
			findingID := stableFindingID(model.Finding{
				RuleID:             obligation.RuleID,
				Severity:           obligation.Severity,
				Confidence:         obligation.Confidence,
				Title:              obligation.Title,
				Why:                obligation.Why,
				Evidence:           obligation.Evidence,
				ExpectedCompanions: normalizedExpectedCompanions(*obligation, options.ExpectedCompanions),
			})
			obligation.FindingID = &findingID
		}
		if !options.PreserveExistingIDs || obligation.OmissionSignature == nil {
			signature := omissionSignature(obligation.RuleID)
			obligation.OmissionSignature = &signature
		}
	}

	return normalized
}

func normalizedExpectedCompanions(obligation model.Obligation, fallback func(model.Obligation) []string) []string {
	if fallback == nil {
		return obligation.ExpectedCompanions
	}
	return fallback(obligation)
}

func importExpectedCompanions(obligation model.Obligation) []string {
	if len(obligation.ExpectedCompanions) > 0 {
		return obligation.ExpectedCompanions
	}

	companions := make([]string, 0, len(obligation.RequiredCompanions))
	for _, companion := range obligation.RequiredCompanions {
		if companion.Kind == "" {
			continue
		}
		companions = append(companions, companion.Kind)
	}
	return companions
}

func withFindingIDs(findings []model.Finding) []model.Finding {
	withIDs := make([]model.Finding, len(findings))
	copy(withIDs, findings)
	for index := range withIDs {
		withIDs[index].FindingID = stableFindingID(withIDs[index])
		withIDs[index].OmissionSignature = omissionSignature(withIDs[index].RuleID)
	}
	return withIDs
}

func withObligationIDs(obligations []model.Obligation) []model.Obligation {
	return normalizeObligations(obligations, obligationNormalizationOptions{})
}

func normalizeObligationArtifactSlices(obligation *model.Obligation) {
	if obligation.Evidence == nil {
		obligation.Evidence = []model.Evidence{}
	}
	if obligation.Anchor.Evidence == nil {
		obligation.Anchor.Evidence = []model.Evidence{}
	}
	if obligation.RequiredCompanions == nil {
		obligation.RequiredCompanions = []model.RequiredCompanion{}
	}
	for index := range obligation.RequiredCompanions {
		if obligation.RequiredCompanions[index].Satisfiers == nil {
			obligation.RequiredCompanions[index].Satisfiers = []string{}
		}
		if obligation.RequiredCompanions[index].SatisfierEvidence == nil {
			obligation.RequiredCompanions[index].SatisfierEvidence = []model.Evidence{}
		}
		if obligation.RequiredCompanions[index].ExpectedPaths == nil {
			obligation.RequiredCompanions[index].ExpectedPaths = []string{}
		}
	}
	if obligation.StatusReason != nil && obligation.StatusReason.Evidence == nil {
		obligation.StatusReason.Evidence = []model.Evidence{}
	}
	if obligation.Suppression != nil && obligation.Suppression.Evidence == nil {
		obligation.Suppression.Evidence = []model.Evidence{}
	}
}

func firstSatisfierEvidence(companions []model.RequiredCompanion) []model.Evidence {
	for _, companion := range companions {
		if len(companion.SatisfierEvidence) > 0 {
			return companion.SatisfierEvidence
		}
	}
	return nil
}

func omissionSignature(ruleID string) string {
	switch ruleID {
	case "DB001":
		return "db001.schema_changed.migration_companion"
	case "DB002":
		return "db002.destructive_storage.rollback_backfill"
	case "API001":
		return "api001.public_api_changed.contract_docs"
	case "CFG001":
		return "cfg001.config_introduced.docs_default"
	case "AUTH001":
		return "auth001.authz_changed.allow_deny"
	case "ERR001":
		return "err001.error_contract_changed.assertion_docs"
	case "OPS001":
		return "ops001.worker_retry_changed.runbook_observability"
	case "DOC001":
		return "doc001.generated_spec_changed.human_explanation"
	default:
		normalizedRuleID := strings.ToLower(strings.TrimSpace(ruleID))
		if normalizedRuleID == "" {
			return "unknown.rule_id.unmapped"
		}
		return normalizedRuleID + ".unmapped"
	}
}

func stableFindingID(finding model.Finding) string {
	hash := sha256.New()
	writeFingerprintField(hash, "version", "specbackfill-finding-v0")
	writeFingerprintField(hash, "rule_id", finding.RuleID)
	writeFingerprintField(hash, "evidence_count", strconv.Itoa(len(finding.Evidence)))
	for index, evidence := range finding.Evidence {
		prefix := fmt.Sprintf("evidence.%d.", index)
		writeFingerprintField(hash, prefix+"file", evidence.File)
		writeFingerprintField(hash, prefix+"line", strconv.Itoa(evidence.Line))
		writeFingerprintField(hash, prefix+"kind", evidence.Kind)
		writeFingerprintField(hash, prefix+"excerpt", evidence.Excerpt)
	}
	writeFingerprintField(hash, "expected_companion_count", strconv.Itoa(len(finding.ExpectedCompanions)))
	for index, companion := range finding.ExpectedCompanions {
		writeFingerprintField(hash, fmt.Sprintf("expected_companion.%d", index), companion)
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))
	return "v0-" + sum[:16]
}

func stableObligationID(obligation model.Obligation) string {
	hash := sha256.New()
	writeFingerprintField(hash, "version", "specbackfill-obligation-v1")
	writeFingerprintField(hash, "rule_id", obligation.RuleID)
	writeFingerprintField(hash, "anchor.kind", obligation.Anchor.Kind)
	writeFingerprintField(hash, "anchor.path", obligation.Anchor.Path)
	if obligation.Anchor.Line != nil {
		writeFingerprintField(hash, "anchor.line", strconv.Itoa(*obligation.Anchor.Line))
	}
	writeEvidenceFingerprint(hash, "anchor.evidence", obligation.Anchor.Evidence)
	writeFingerprintField(hash, "required_companion_count", strconv.Itoa(len(obligation.RequiredCompanions)))
	for index, companion := range obligation.RequiredCompanions {
		prefix := fmt.Sprintf("required_companion.%d.", index)
		writeFingerprintField(hash, prefix+"kind", companion.Kind)
		for pathIndex, expectedPath := range companion.ExpectedPaths {
			writeFingerprintField(hash, fmt.Sprintf("%sexpected_path.%d", prefix, pathIndex), expectedPath)
		}
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))
	return "obl-v1-" + sum[:16]
}

func writeEvidenceFingerprint(hash io.Writer, prefix string, evidence []model.Evidence) {
	writeFingerprintField(hash, prefix+".count", strconv.Itoa(len(evidence)))
	for index, item := range evidence {
		itemPrefix := fmt.Sprintf("%s.%d.", prefix, index)
		writeFingerprintField(hash, itemPrefix+"file", item.File)
		writeFingerprintField(hash, itemPrefix+"line", strconv.Itoa(item.Line))
		writeFingerprintField(hash, itemPrefix+"kind", item.Kind)
		writeFingerprintField(hash, itemPrefix+"excerpt", item.Excerpt)
	}
}

func stableEvidenceDigest(obligation model.Obligation) string {
	hash := sha256.New()
	writeFingerprintField(hash, "version", "specbackfill-local-ai-review-evidence-v1")
	writeEvidenceFingerprint(hash, "anchor.evidence", obligation.Anchor.Evidence)
	writeEvidenceFingerprint(hash, "obligation.evidence", obligation.Evidence)
	writeFingerprintField(hash, "required_companion_count", strconv.Itoa(len(obligation.RequiredCompanions)))
	for index, companion := range obligation.RequiredCompanions {
		prefix := fmt.Sprintf("required_companion.%d.", index)
		writeEvidenceFingerprint(hash, prefix+"satisfier_evidence", companion.SatisfierEvidence)
	}
	if obligation.StatusReason == nil {
		writeFingerprintField(hash, "status_reason", "")
	} else {
		writeFingerprintField(hash, "status_reason.reason", string(obligation.StatusReason.Reason))
		writeEvidenceFingerprint(hash, "status_reason.evidence", obligation.StatusReason.Evidence)
	}
	if obligation.Suppression == nil {
		writeFingerprintField(hash, "suppression", "")
	} else {
		writeFingerprintField(hash, "suppression.reason", string(obligation.Suppression.Reason))
		writeEvidenceFingerprint(hash, "suppression.evidence", obligation.Suppression.Evidence)
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))
	return "sha256:" + sum
}

func diffFingerprint(input []byte) string {
	sum := sha256.Sum256(input)
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeFingerprintField(w io.Writer, name, value string) {
	_, _ = io.WriteString(w, name)
	_, _ = io.WriteString(w, "\x00")
	_, _ = io.WriteString(w, value)
	_, _ = io.WriteString(w, "\x00")
}

func Write(w io.Writer, format string, diff model.Diff, result model.Report) error {
	return WriteWithOptions(w, format, diff, result, Options{})
}

func WriteWithOptions(w io.Writer, format string, diff model.Diff, result model.Report, options Options) error {
	if options.SummaryOnly {
		return writeSummary(w, format, diff, result, options)
	}

	switch format {
	case "text":
		return writeText(w, diff, result, options)
	case "json":
		return writeJSON(w, result)
	case "markdown":
		return writeMarkdown(w, diff, result, options)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func ExitCode(findings []model.Finding, failOn string) int {
	switch failOn {
	case "off":
		return 0
	case "warn":
		for _, finding := range findings {
			if finding.Severity == model.SeverityError || finding.Severity == model.SeverityWarn {
				return 1
			}
		}
		return 0
	case "error":
		for _, finding := range findings {
			if finding.Severity == model.SeverityError {
				return 1
			}
		}
		return 0
	default:
		return 2
	}
}

func writeText(w io.Writer, diff model.Diff, result model.Report, options Options) error {
	if _, err := fmt.Fprintln(w, "specbackfill check"); err != nil {
		return err
	}
	if err := writeTextInputSummary(w, options); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if err := writeTextFileSummary(w, diff); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "findings: error=%d warn=%d info=%d\n", result.Summary.Error, result.Summary.Warn, result.Summary.Info); err != nil {
		return err
	}

	labels := result.RepoProfile.Labels()
	if len(labels) > 0 {
		if _, err := fmt.Fprintf(w, "repo profile: %s\n", strings.Join(labels, ", ")); err != nil {
			return err
		}
	}

	if len(result.Findings) == 0 {
		if _, err := fmt.Fprintln(w, "No findings emitted."); err != nil {
			return err
		}
		if err := writeTextAnchorScan(w, options); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "This means no implemented v0 companion-artifact rule fired for this diff; it does not prove the diff is complete.")
		return err
	}

	for index, finding := range result.Findings {
		if index > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "[%s] %s %s\n", finding.Severity, finding.RuleID, finding.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "  why: %s\n", finding.Why); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "  evidence:"); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "    - %s\n", formatEvidence(evidence)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "  expected companions:"); err != nil {
			return err
		}
		for _, companion := range finding.ExpectedCompanions {
			if _, err := fmt.Fprintf(w, "    - %s\n", companion); err != nil {
				return err
			}
		}
		if explanation, ok := explanationForFinding(result.Explanations, index, finding.RuleID); ok {
			if _, err := fmt.Fprintf(w, "  explanation: %s\n", explanation.Summary); err != nil {
				return err
			}
		}
	}

	return nil
}

func explanationForFinding(explanations []model.Explanation, index int, ruleID string) (model.Explanation, bool) {
	for _, explanation := range explanations {
		if explanation.FindingIndex == index && explanation.RuleID == ruleID {
			return explanation, true
		}
	}
	return model.Explanation{}, false
}

func writeTextInputSummary(w io.Writer, options Options) error {
	if options.InputSummary == "" {
		return nil
	}
	if _, err := fmt.Fprintf(w, "input: %s\n", options.InputSummary); err != nil {
		return err
	}
	for _, note := range options.InputNotes {
		if _, err := fmt.Fprintf(w, "note: %s\n", note); err != nil {
			return err
		}
	}
	return nil
}

func writeTextFileSummary(w io.Writer, diff model.Diff) error {
	rows := fileSummaryRows(diff)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "changed file summary: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "changed file summary:"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "  - %s: %d\n", row.label, row.count); err != nil {
			return err
		}
		for _, sample := range row.samples {
			if _, err := fmt.Fprintf(w, "    - %s\n", sample); err != nil {
				return err
			}
		}
		if remaining := row.count - len(row.samples); remaining > 0 {
			if _, err := fmt.Fprintf(w, "    - ... %d more\n", remaining); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeTextAnchorScan(w io.Writer, options Options) error {
	if !options.AnchorScanAvailable {
		return nil
	}
	if len(options.AnchorRuleIDs) == 0 {
		_, err := fmt.Fprintln(w, "anchor scan: no v0 anchor evidence matched.")
		return err
	}
	_, err := fmt.Fprintf(w, "anchor scan: %s evidence matched, but no finding remained after companion/suppression checks.\n", strings.Join(options.AnchorRuleIDs, ", "))
	return err
}

func writeJSON(w io.Writer, result model.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteObligationArtifact(w io.Writer, artifact model.ObligationArtifact) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(artifact)
}

func WriteLocalAIReviewImport(w io.Writer, items []model.LocalAIReviewImportItem) error {
	encoder := json.NewEncoder(w)
	for _, item := range items {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func WriteTodo(w io.Writer, format string, obligations []model.Obligation, options Options) error {
	switch format {
	case "text":
		return writeTextTodo(w, obligations, options)
	case "markdown":
		return writeMarkdownTodo(w, obligations, options)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeTextTodo(w io.Writer, obligations []model.Obligation, options Options) error {
	missing := missingObligations(obligations)
	if _, err := fmt.Fprintln(w, "specbackfill todo"); err != nil {
		return err
	}
	if err := writeTextInputSummary(w, options); err != nil {
		return err
	}
	count := len(missing)
	obligationLabel := "obligations"
	if count == 1 {
		obligationLabel = "obligation"
	}
	if _, err := fmt.Fprintf(w, "%d unresolved %s\n", count, obligationLabel); err != nil {
		return err
	}
	if count == 0 {
		if _, err := fmt.Fprintln(w, "No next actions from implemented v0 rules."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "This does not prove the diff is complete.")
		return err
	}

	for index, obligation := range missing {
		if index > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		item := todoItemForObligation(obligation)
		if _, err := fmt.Fprintf(w, "[%s] %s %s\n", obligation.Severity, obligation.RuleID, item.relation); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Anchor: %s\n", item.anchor); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Missing: %s\n", item.missing); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Next: %s\n", item.next); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownTodo(w io.Writer, obligations []model.Obligation, options Options) error {
	missing := missingObligations(obligations)
	if _, err := fmt.Fprintln(w, "### specbackfill todo"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writeMarkdownInputSummary(w, options); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Unresolved obligations: %d\n", len(missing)); err != nil {
		return err
	}
	if len(missing) == 0 {
		if _, err := fmt.Fprintln(w, "\nNo next actions from implemented v0 rules."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "\nThis does not prove the diff is complete.")
		return err
	}

	for _, obligation := range missing {
		item := todoItemForObligation(obligation)
		if _, err := fmt.Fprintf(w, "\n#### [%s] %s %s\n\n", obligation.Severity, obligation.RuleID, item.relation); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Anchor: %s\n", markdownCode(item.anchor)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Missing: %s\n", item.missing); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Next: %s\n", item.next); err != nil {
			return err
		}
	}
	return nil
}

func missingObligations(obligations []model.Obligation) []model.Obligation {
	missing := make([]model.Obligation, 0, len(obligations))
	for _, obligation := range obligations {
		if obligation.Status == model.ObligationStatusMissing {
			missing = append(missing, obligation)
		}
	}
	return missing
}

type todoItem struct {
	relation string
	anchor   string
	missing  string
	next     string
}

func todoItemForObligation(obligation model.Obligation) todoItem {
	copy := todoCopyForRule(obligation.RuleID)
	return todoItem{
		relation: copy.relation,
		anchor:   todoAnchor(obligation),
		missing:  copy.missing,
		next:     copy.next,
	}
}

type todoCopy struct {
	relation string
	missing  string
	next     string
}

func todoCopyForRule(ruleID string) todoCopy {
	switch ruleID {
	case "DB001":
		return todoCopy{
			relation: "schema -> migration",
			missing:  "migration file companion did not move in this diff",
			next:     "add a matching migration file, or record why this diff intentionally has no migration companion",
		}
	case "DB002":
		return todoCopy{
			relation: "destructive storage -> rollback/backfill",
			missing:  "rollback, backfill, or compatibility companion did not move in this diff",
			next:     "add rollback/backfill notes or compatibility coverage, or record why this diff intentionally has none",
		}
	case "API001":
		return todoCopy{
			relation: "public API -> contract/docs",
			missing:  "contract test, API docs, or compatibility note companion did not move in this diff",
			next:     "add/update contract tests, API docs, or compatibility/deprecation notes",
		}
	case "CFG001":
		return todoCopy{
			relation: "config/env -> docs/default",
			missing:  "docs, default handling, or upgrade note companion did not move in this diff",
			next:     "add/update docs, defaults, or upgrade notes for the new config surface",
		}
	case "AUTH001":
		return todoCopy{
			relation: "authz -> allow/deny evidence",
			missing:  "allow/deny test or security-sensitive note companion did not move in this diff",
			next:     "add allow/deny coverage or a security-sensitive note tied to the auth change",
		}
	case "ERR001":
		return todoCopy{
			relation: "error/status -> assertion/docs",
			missing:  "assertion test, API note, or client note companion did not move in this diff",
			next:     "add/update assertion coverage or document the public error/status change",
		}
	case "OPS001":
		return todoCopy{
			relation: "worker/retry -> runbook/observability",
			missing:  "runbook, observability, or rollback companion did not move in this diff",
			next:     "add/update runbook, alerting/observability notes, or rollback guidance",
		}
	case "DOC001":
		return todoCopy{
			relation: "generated spec -> human explanation",
			missing:  "human-facing docs or upgrade note companion did not move in this diff",
			next:     "add/update human-facing explanation for the generated spec/client change",
		}
	default:
		return todoCopy{
			relation: "change -> companion work",
			missing:  "expected companion evidence did not move in this diff",
			next:     "add/update the companion artifact, or record why this diff intentionally has none",
		}
	}
}

func todoAnchor(obligation model.Obligation) string {
	evidence := firstTodoEvidence(obligation)
	if evidence.File == "" {
		if obligation.Anchor.Path != "" {
			return obligation.Anchor.Path
		}
		return "unknown diff evidence"
	}

	location := evidence.File
	marker := todoLineMarker(evidence.Kind)
	if evidence.Line > 0 && marker != "" {
		location = fmt.Sprintf("%s %s%d", location, marker, evidence.Line)
	} else if evidence.Line > 0 {
		location = fmt.Sprintf("%s %d", location, evidence.Line)
	}

	excerpt := strings.TrimSpace(evidence.Excerpt)
	if excerpt == "" {
		return location
	}
	return fmt.Sprintf("%s: %s", location, excerpt)
}

func firstTodoEvidence(obligation model.Obligation) model.Evidence {
	if len(obligation.Anchor.Evidence) > 0 {
		return obligation.Anchor.Evidence[0]
	}
	if len(obligation.Evidence) > 0 {
		return obligation.Evidence[0]
	}
	return model.Evidence{}
}

func todoLineMarker(kind string) string {
	switch kind {
	case string(model.LineKindAdded):
		return "+"
	case string(model.LineKindRemoved):
		return "-"
	case string(model.LineKindContext):
		return "~"
	default:
		return ""
	}
}

func writeMarkdown(w io.Writer, diff model.Diff, result model.Report, options Options) error {
	if _, err := fmt.Fprintln(w, "### specbackfill findings"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writeMarkdownInputSummary(w, options); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if err := writeMarkdownFileSummary(w, diff); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Findings: error=%d warn=%d info=%d\n", result.Summary.Error, result.Summary.Warn, result.Summary.Info); err != nil {
		return err
	}

	labels := result.RepoProfile.Labels()
	if len(labels) > 0 {
		if _, err := fmt.Fprintf(w, "- Repo profile: %s\n", strings.Join(labels, ", ")); err != nil {
			return err
		}
	}

	if len(result.Findings) == 0 {
		if _, err := fmt.Fprintln(w, "\nNo findings emitted."); err != nil {
			return err
		}
		if err := writeMarkdownAnchorScan(w, options); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "\nThis means no implemented v0 companion-artifact rule fired for this diff; it does not prove the diff is complete.")
		return err
	}

	for index, finding := range result.Findings {
		if _, err := fmt.Fprintf(w, "\n#### [%s] %s %s\n\n", finding.Severity, finding.RuleID, finding.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, finding.Why); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\nEvidence:"); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "- %s\n", formatMarkdownEvidence(evidence)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\nExpected companions:"); err != nil {
			return err
		}
		for _, companion := range finding.ExpectedCompanions {
			if _, err := fmt.Fprintf(w, "- %s\n", companion); err != nil {
				return err
			}
		}
		if explanation, ok := explanationForFinding(result.Explanations, index, finding.RuleID); ok {
			if _, err := fmt.Fprintf(w, "\nExplanation: %s\n", explanation.Summary); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeMarkdownInputSummary(w io.Writer, options Options) error {
	if options.InputSummary == "" {
		return nil
	}
	if _, err := fmt.Fprintf(w, "- Input: %s\n", options.InputSummary); err != nil {
		return err
	}
	for _, note := range options.InputNotes {
		if _, err := fmt.Fprintf(w, "- Note: %s\n", note); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownFileSummary(w io.Writer, diff model.Diff) error {
	rows := fileSummaryRows(diff)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "- Changed file summary: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "- Changed file summary:"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "  - %s: %d\n", row.label, row.count); err != nil {
			return err
		}
		for _, sample := range row.samples {
			if _, err := fmt.Fprintf(w, "    - %s\n", markdownCode(sample)); err != nil {
				return err
			}
		}
		if remaining := row.count - len(row.samples); remaining > 0 {
			if _, err := fmt.Fprintf(w, "    - ... %d more\n", remaining); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMarkdownAnchorScan(w io.Writer, options Options) error {
	if !options.AnchorScanAvailable {
		return nil
	}
	if len(options.AnchorRuleIDs) == 0 {
		_, err := fmt.Fprintln(w, "\nAnchor scan: no v0 anchor evidence matched.")
		return err
	}
	_, err := fmt.Fprintf(w, "\nAnchor scan: `%s` evidence matched, but no finding remained after companion/suppression checks.\n", strings.Join(options.AnchorRuleIDs, "`, `"))
	return err
}

func writeSummary(w io.Writer, format string, diff model.Diff, result model.Report, options Options) error {
	switch format {
	case "text":
		return writeTextSummary(w, diff, result, options)
	case "json":
		return writeJSONSummary(w, diff, result)
	case "markdown":
		return writeMarkdownSummary(w, diff, result, options)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeTextSummary(w io.Writer, diff model.Diff, result model.Report, options Options) error {
	if _, err := fmt.Fprintln(w, "specbackfill summary"); err != nil {
		return err
	}
	if err := writeTextInputSummary(w, options); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if err := writeTextFileSummary(w, diff); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "error: %d\n", result.Summary.Error); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "warn:  %d\n", result.Summary.Warn); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "info:  %d\n", result.Summary.Info); err != nil {
		return err
	}
	return writeTextRuleCounts(w, result.Findings)
}

func writeTextRuleCounts(w io.Writer, findings []model.Finding) error {
	counts, order := ruleCounts(findings)
	if len(order) == 0 {
		_, err := fmt.Fprintln(w, "\nRules fired: none")
		return err
	}
	if _, err := fmt.Fprintln(w, "\nRules fired:"); err != nil {
		return err
	}
	for _, ruleID := range order {
		if _, err := fmt.Fprintf(w, "- %s: %d\n", ruleID, counts[ruleID]); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownSummary(w io.Writer, diff model.Diff, result model.Report, options Options) error {
	if _, err := fmt.Fprintln(w, "### specbackfill summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writeMarkdownInputSummary(w, options); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Changed files: %d\n", len(diff.Files)); err != nil {
		return err
	}
	if err := writeMarkdownFileSummary(w, diff); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Error: %d\n", result.Summary.Error); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Warn: %d\n", result.Summary.Warn); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Info: %d\n", result.Summary.Info); err != nil {
		return err
	}

	counts, order := ruleCounts(result.Findings)
	if len(order) == 0 {
		_, err := fmt.Fprintln(w, "\n#### Rules fired\n\nNone.")
		return err
	}
	if _, err := fmt.Fprintln(w, "\n#### Rules fired"); err != nil {
		return err
	}
	for _, ruleID := range order {
		if _, err := fmt.Fprintf(w, "- `%s`: %d\n", ruleID, counts[ruleID]); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONSummary(w io.Writer, diff model.Diff, result model.Report) error {
	counts, _ := ruleCounts(result.Findings)
	payload := struct {
		Version      string            `json:"version"`
		ChangedFiles int               `json:"changed_files"`
		Summary      model.Summary     `json:"summary"`
		RulesFired   map[string]int    `json:"rules_fired"`
		RepoProfile  model.RepoProfile `json:"repo_profile"`
	}{
		Version:      result.Version,
		ChangedFiles: len(diff.Files),
		Summary:      result.Summary,
		RulesFired:   counts,
		RepoProfile:  result.RepoProfile,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

type fileSummaryRow struct {
	label   string
	count   int
	samples []string
}

const maxFileSummarySamples = 3

func fileSummaryRows(diff model.Diff) []fileSummaryRow {
	counts := map[string]int{}
	samples := map[string][]string{}
	for _, file := range diff.Files {
		label := classifyFile(file.Path)
		counts[label]++
		if len(samples[label]) < maxFileSummarySamples {
			samples[label] = append(samples[label], file.Path)
		}
	}

	rows := make([]fileSummaryRow, 0, len(fileSummaryOrder))
	for _, label := range fileSummaryOrder {
		count := counts[label]
		if count == 0 {
			continue
		}
		rows = append(rows, fileSummaryRow{label: label, count: count, samples: samples[label]})
	}
	return rows
}

var fileSummaryOrder = []string{
	"db schema",
	"migrations",
	"API specs",
	"generated",
	"docs",
	"tests",
	"test fixtures",
	"config/ci",
	"scripts",
	"Go source",
	"Rust source",
	"TypeScript source",
	"JavaScript source",
	"Python source",
	"SQL",
	"examples/samples",
	"other",
}

func classifyFile(filePath string) string {
	lower := strings.ToLower(filePath)
	base := baseName(lower)

	switch {
	case lower == "schema.prisma" || lower == "db/schema.sql" || strings.HasPrefix(lower, "ent/schema/") || strings.HasPrefix(lower, "sqlc/schema/"):
		return "db schema"
	case strings.HasPrefix(lower, "migrations/") || strings.HasPrefix(lower, "db/migrations/") || strings.HasPrefix(lower, "prisma/migrations/"):
		return "migrations"
	case base == "schema.graphql" ||
		((strings.HasPrefix(base, "openapi") || strings.Contains(lower, "/openapi/")) && (strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml"))) ||
		(strings.HasSuffix(lower, ".proto") && (strings.HasPrefix(lower, "proto/") || strings.Contains(lower, "/proto/"))):
		return "API specs"
	case strings.HasPrefix(lower, "testdata/"):
		return "test fixtures"
	case isSummaryExamplePath(lower):
		return "examples/samples"
	case isSummaryGeneratedPath(lower):
		return "generated"
	case isSummaryDocsPath(lower, base):
		return "docs"
	case isSummaryTestPath(lower, base):
		return "tests"
	case isSummaryConfigPath(lower, base):
		return "config/ci"
	case strings.HasPrefix(lower, "scripts/") || strings.HasPrefix(lower, "bin/") || strings.HasSuffix(base, ".sh"):
		return "scripts"
	case strings.HasSuffix(base, ".go"):
		return "Go source"
	case strings.HasSuffix(base, ".rs"):
		return "Rust source"
	case strings.HasSuffix(base, ".ts") || strings.HasSuffix(base, ".tsx"):
		return "TypeScript source"
	case strings.HasSuffix(base, ".js") || strings.HasSuffix(base, ".jsx") || strings.HasSuffix(base, ".mjs") || strings.HasSuffix(base, ".cjs"):
		return "JavaScript source"
	case strings.HasSuffix(base, ".py"):
		return "Python source"
	case strings.HasSuffix(base, ".sql"):
		return "SQL"
	default:
		return "other"
	}
}

func baseName(filePath string) string {
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash == -1 {
		return filePath
	}
	return filePath[lastSlash+1:]
}

func isSummaryGeneratedPath(filePath string) bool {
	base := baseName(filePath)
	return strings.HasSuffix(base, ".pb.go") ||
		strings.Contains(base, ".generated.") ||
		strings.HasPrefix(base, "generated_") ||
		hasSummaryPathSegment(filePath, "generated") ||
		hasSummaryPathSegment(filePath, "gen")
}

func isSummaryDocsPath(filePath, base string) bool {
	return strings.HasPrefix(filePath, "docs/") ||
		strings.HasPrefix(base, "readme") ||
		strings.HasPrefix(base, "changelog") ||
		strings.HasPrefix(base, "upgrade") ||
		strings.HasSuffix(base, ".md") ||
		strings.HasSuffix(base, ".mdx") ||
		strings.HasSuffix(base, ".rst")
}

func isSummaryTestPath(filePath, base string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		hasSummaryPathSegment(filePath, "tests") ||
		hasSummaryPathSegment(filePath, "test") ||
		hasSummaryPathSegment(filePath, "integration") ||
		hasSummaryPathSegment(filePath, "contract") ||
		hasSummaryPathSegment(filePath, "e2e")
}

func isSummaryConfigPath(filePath, base string) bool {
	return strings.HasPrefix(filePath, ".github/") ||
		strings.HasPrefix(filePath, ".gitlab/") ||
		strings.HasPrefix(filePath, "configs/") ||
		strings.HasPrefix(filePath, "config/") ||
		base == ".gitlab-ci.yml" ||
		base == ".gitlab-ci.yaml" ||
		base == ".mise.toml" ||
		base == "makefile" ||
		base == "justfile" ||
		base == "go.mod" ||
		base == "go.sum" ||
		base == "cargo.toml" ||
		base == "cargo.lock" ||
		base == "package.json" ||
		base == "package-lock.json" ||
		base == "pnpm-lock.yaml" ||
		base == "yarn.lock" ||
		base == "tsconfig.json" ||
		strings.HasSuffix(base, ".toml") ||
		strings.HasSuffix(base, ".yaml") ||
		strings.HasSuffix(base, ".yml") ||
		strings.HasSuffix(base, ".json")
}

func isSummaryExamplePath(filePath string) bool {
	return hasSummaryPathSegment(filePath, "examples") ||
		hasSummaryPathSegment(filePath, "example") ||
		hasTopLevelSummaryPathSegment(filePath, "samples") ||
		hasTopLevelSummaryPathSegment(filePath, "sample")
}

func hasSummaryPathSegment(filePath, segment string) bool {
	return filePath == segment ||
		strings.HasPrefix(filePath, segment+"/") ||
		strings.Contains(filePath, "/"+segment+"/") ||
		strings.HasSuffix(filePath, "/"+segment)
}

func hasTopLevelSummaryPathSegment(filePath, segment string) bool {
	return filePath == segment || strings.HasPrefix(filePath, segment+"/")
}

func formatEvidence(evidence model.Evidence) string {
	kind := evidence.Kind
	switch evidence.Kind {
	case string(model.LineKindAdded):
		kind = "+"
	case string(model.LineKindRemoved):
		kind = "-"
	case string(model.LineKindContext):
		kind = "~"
	}

	return fmt.Sprintf("%s:%s %s", evidence.File, kind, evidence.Excerpt)
}

func formatMarkdownEvidence(evidence model.Evidence) string {
	kind := evidence.Kind
	switch evidence.Kind {
	case string(model.LineKindAdded):
		kind = "+"
	case string(model.LineKindRemoved):
		kind = "-"
	case string(model.LineKindContext):
		kind = "~"
	}

	return fmt.Sprintf("%s: %s", markdownCode(evidence.File), markdownCode(kind+" "+evidence.Excerpt))
}

func markdownCode(value string) string {
	longestRun := 0
	currentRun := 0
	for _, r := range value {
		if r == '`' {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
			continue
		}
		currentRun = 0
	}

	fence := strings.Repeat("`", longestRun+1)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		return fence + " " + value + " " + fence
	}
	return fence + value + fence
}

func ruleCounts(findings []model.Finding) (map[string]int, []string) {
	counts := map[string]int{}
	order := make([]string, 0, len(findings))
	for _, finding := range findings {
		if _, ok := counts[finding.RuleID]; !ok {
			order = append(order, finding.RuleID)
		}
		counts[finding.RuleID]++
	}
	return counts, order
}
