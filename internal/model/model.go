package model

import (
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
