package model

import (
	"encoding/json"
	"path"
	"strings"
)

type FileStatus string

const (
	FileStatusUnknown  FileStatus = "unknown"
	FileStatusAdded    FileStatus = "added"
	FileStatusModified FileStatus = "modified"
	FileStatusDeleted  FileStatus = "deleted"
	FileStatusRenamed  FileStatus = "renamed"
	FileStatusCopied   FileStatus = "copied"
)

type LineKind string

const (
	LineKindContext LineKind = "context"
	LineKindAdded   LineKind = "added"
	LineKindRemoved LineKind = "removed"
)

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

type ObligationStatus string

const (
	ObligationStatusSatisfied  ObligationStatus = "satisfied"
	ObligationStatusMissing    ObligationStatus = "missing"
	ObligationStatusUnknown    ObligationStatus = "unknown"
	ObligationStatusSuppressed ObligationStatus = "suppressed"
)

type SuppressionReason string

const (
	SuppressionReasonDocsOnly      SuppressionReason = "docs_only"
	SuppressionReasonTestsOnly     SuppressionReason = "tests_only"
	SuppressionReasonExampleOnly   SuppressionReason = "example_only"
	SuppressionReasonSampleOnly    SuppressionReason = "sample_only"
	SuppressionReasonGeneratedOnly SuppressionReason = "generated_only"
	SuppressionReasonMigrationOnly SuppressionReason = "migration_only"
)

type StatusReason string

const (
	StatusReasonCompanionPresent StatusReason = "companion_present"
	StatusReasonDocsOnly         StatusReason = "docs_only"
	StatusReasonTestsOnly        StatusReason = "tests_only"
	StatusReasonExampleOnly      StatusReason = "example_only"
	StatusReasonSampleOnly       StatusReason = "sample_only"
	StatusReasonGeneratedOnly    StatusReason = "generated_only"
	StatusReasonMigrationOnly    StatusReason = "migration_only"
)

type Diff struct {
	Files []File
}

type File struct {
	Path    string
	OldPath string
	NewPath string
	Status  FileStatus
	Hunks   []Hunk
}

type Hunk struct {
	Header   string
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []Line
}

type Line struct {
	Kind    LineKind
	Text    string
	OldLine int
	NewLine int
}

type RepoProfile struct {
	Go         bool `json:"go"`
	Node       bool `json:"node"`
	Prisma     bool `json:"prisma"`
	OpenAPI    bool `json:"openapi"`
	Proto      bool `json:"proto"`
	Migrations bool `json:"migrations"`
}

func (p RepoProfile) Labels() []string {
	labels := make([]string, 0, 6)
	if p.Go {
		labels = append(labels, "go")
	}
	if p.Node {
		labels = append(labels, "node")
	}
	if p.Prisma {
		labels = append(labels, "prisma")
	}
	if p.OpenAPI {
		labels = append(labels, "openapi")
	}
	if p.Proto {
		labels = append(labels, "proto")
	}
	if p.Migrations {
		labels = append(labels, "migrations")
	}
	return labels
}

type Report struct {
	Version      string        `json:"version"`
	Summary      Summary       `json:"summary"`
	RepoProfile  RepoProfile   `json:"repo_profile"`
	Findings     []Finding     `json:"findings"`
	Explanations []Explanation `json:"explanations,omitempty"`
}

type Summary struct {
	Error int `json:"error"`
	Warn  int `json:"warn"`
	Info  int `json:"info"`
}

type Finding struct {
	FindingID          string     `json:"finding_id,omitempty"`
	OmissionSignature  string     `json:"omission_signature,omitempty"`
	RuleID             string     `json:"rule_id"`
	Severity           Severity   `json:"severity"`
	Confidence         string     `json:"confidence"`
	Title              string     `json:"title"`
	Why                string     `json:"why"`
	Evidence           []Evidence `json:"evidence"`
	ExpectedCompanions []string   `json:"expected_companions"`
}

type Explanation struct {
	FindingIndex       int        `json:"finding_index"`
	RuleID             string     `json:"rule_id"`
	Source             string     `json:"source"`
	Summary            string     `json:"summary"`
	Evidence           []Evidence `json:"evidence"`
	ExpectedCompanions []string   `json:"expected_companions"`
}

type ObligationArtifact struct {
	SchemaVersion string       `json:"schema_version"`
	Tool          ToolMetadata `json:"tool"`
	Run           RunMetadata  `json:"run"`
	Obligations   []Obligation `json:"obligations"`
}

type ToolMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RunMetadata struct {
	RunID           string  `json:"run_id"`
	InputKind       string  `json:"input_kind"`
	Base            *string `json:"base"`
	Head            *string `json:"head"`
	DiffFingerprint string  `json:"diff_fingerprint"`
}

type Obligation struct {
	ObligationID       string                  `json:"obligation_id"`
	FindingID          *string                 `json:"finding_id"`
	OmissionSignature  *string                 `json:"omission_signature"`
	RuleID             string                  `json:"rule_id"`
	RuleVersion        string                  `json:"rule_version"`
	Status             ObligationStatus        `json:"status"`
	Severity           Severity                `json:"severity"`
	Confidence         string                  `json:"confidence"`
	Title              string                  `json:"title"`
	Why                string                  `json:"why"`
	DiffLocalClaim     bool                    `json:"diff_local_claim"`
	Anchor             ObligationAnchor        `json:"anchor"`
	RequiredCompanions []RequiredCompanion     `json:"required_companions"`
	Evidence           []Evidence              `json:"evidence"`
	StatusReason       *ObligationStatusReason `json:"status_reason,omitempty"`
	Suppression        *ObligationSuppression  `json:"suppression"`
	Downstream         DownstreamMetadata      `json:"downstream"`
	ExpectedCompanions []string                `json:"-"`
}

type ObligationAnchor struct {
	Kind     string     `json:"kind"`
	Path     string     `json:"path"`
	Line     *int       `json:"line"`
	Evidence []Evidence `json:"evidence"`
}

type RequiredCompanion struct {
	Kind              string           `json:"kind"`
	Status            ObligationStatus `json:"status"`
	Satisfiers        []string         `json:"satisfiers"`
	SatisfierEvidence []Evidence       `json:"satisfier_evidence"`
	ExpectedPaths     []string         `json:"expected_paths"`
}

type ObligationStatusReason struct {
	Reason   StatusReason `json:"reason"`
	Evidence []Evidence   `json:"evidence"`
}

type ObligationSuppression struct {
	Reason   SuppressionReason `json:"reason"`
	Evidence []Evidence        `json:"evidence"`
}

type DownstreamMetadata struct {
	ImportKind   string `json:"import_kind"`
	SourceSignal string `json:"source_signal"`
}

type LocalAIReviewImportItem struct {
	SchemaVersion      string                  `json:"schema_version"`
	Source             string                  `json:"source"`
	ImportKind         string                  `json:"import_kind"`
	SourceSignal       string                  `json:"source_signal"`
	ToolVersion        string                  `json:"tool_version"`
	RunID              string                  `json:"run_id"`
	InputKind          string                  `json:"input_kind"`
	DiffFingerprint    string                  `json:"diff_fingerprint"`
	ItemID             string                  `json:"item_id"`
	ObligationID       string                  `json:"obligation_id"`
	FindingID          *string                 `json:"finding_id"`
	OmissionSignature  *string                 `json:"omission_signature"`
	RuleID             string                  `json:"rule_id"`
	RuleVersion        string                  `json:"rule_version"`
	Status             ObligationStatus        `json:"status"`
	Severity           Severity                `json:"severity"`
	Confidence         string                  `json:"confidence"`
	Title              string                  `json:"title"`
	Why                string                  `json:"why"`
	DiffLocalClaim     bool                    `json:"diff_local_claim"`
	EvidenceDigest     string                  `json:"evidence_digest"`
	Anchor             ObligationAnchor        `json:"anchor"`
	RequiredCompanions []RequiredCompanion     `json:"required_companions"`
	Evidence           []Evidence              `json:"evidence"`
	StatusReason       *ObligationStatusReason `json:"status_reason"`
	Suppression        *ObligationSuppression  `json:"suppression"`
	RawJSON            json.RawMessage         `json:"raw_json"`
}

type Evidence struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Excerpt string `json:"excerpt"`
}

func NormalizePath(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if normalized == "" || normalized == "/dev/null" {
		return ""
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func SummaryFromFindings(findings []Finding) Summary {
	var summary Summary
	for _, finding := range findings {
		switch finding.Severity {
		case SeverityError:
			summary.Error++
		case SeverityWarn:
			summary.Warn++
		case SeverityInfo:
			summary.Info++
		}
	}
	return summary
}
