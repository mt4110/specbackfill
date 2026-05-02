package diffparse

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mt4110/specbackfill/internal/model"
)

var (
	ErrMalformedDiff = errors.New("malformed unified diff")
	hunkHeaderRE     = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)
)

func Parse(data []byte) (model.Diff, error) {
	normalized := normalizeNewlines(data)
	if len(bytes.TrimSpace(normalized)) == 0 {
		return model.Diff{}, nil
	}

	var diff model.Diff
	var currentFile *model.File
	var currentHunk *model.Hunk
	var nextOldLine int
	var nextNewLine int
	var remainingOldLines int
	var remainingNewLines int
	var currentStarted bool

	hunkComplete := func() bool {
		return remainingOldLines == 0 && remainingNewLines == 0
	}

	flushHunk := func() {
		if currentFile == nil || currentHunk == nil {
			return
		}
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		currentHunk = nil
		remainingOldLines = 0
		remainingNewLines = 0
	}

	flushFile := func() error {
		if currentFile == nil {
			return nil
		}
		if currentHunk != nil && !hunkComplete() {
			return fmt.Errorf("%w: truncated hunk %q", ErrMalformedDiff, currentHunk.Header)
		}
		flushHunk()
		finalizeFile(currentFile)
		if currentFile.Path != "" || len(currentFile.Hunks) > 0 {
			diff.Files = append(diff.Files, *currentFile)
		}
		currentFile = nil
		currentStarted = false
		return nil
	}

	for _, line := range splitLines(normalized) {
		if currentHunk != nil && hunkComplete() {
			flushHunk()
		}

		if currentHunk != nil {
			switch {
			case strings.HasPrefix(line, "@@ "):
				if !hunkComplete() {
					return model.Diff{}, fmt.Errorf("%w: truncated hunk before %q", ErrMalformedDiff, line)
				}
				flushHunk()
				hunk, err := parseHunkHeader(line)
				if err != nil {
					return model.Diff{}, err
				}
				currentHunk = &hunk
				nextOldLine = hunk.OldStart
				nextNewLine = hunk.NewStart
				remainingOldLines = hunk.OldLines
				remainingNewLines = hunk.NewLines
				continue
			case strings.HasPrefix(line, " "):
				if remainingOldLines <= 0 || remainingNewLines <= 0 {
					return model.Diff{}, fmt.Errorf("%w: too many context lines in hunk %q", ErrMalformedDiff, currentHunk.Header)
				}
				currentHunk.Lines = append(currentHunk.Lines, model.Line{
					Kind:    model.LineKindContext,
					Text:    line[1:],
					OldLine: nextOldLine,
					NewLine: nextNewLine,
				})
				nextOldLine++
				nextNewLine++
				remainingOldLines--
				remainingNewLines--
				continue
			case strings.HasPrefix(line, "+"):
				if remainingNewLines <= 0 {
					return model.Diff{}, fmt.Errorf("%w: too many added lines in hunk %q", ErrMalformedDiff, currentHunk.Header)
				}
				currentHunk.Lines = append(currentHunk.Lines, model.Line{
					Kind:    model.LineKindAdded,
					Text:    line[1:],
					NewLine: nextNewLine,
				})
				nextNewLine++
				remainingNewLines--
				continue
			case strings.HasPrefix(line, "-"):
				if remainingOldLines <= 0 {
					return model.Diff{}, fmt.Errorf("%w: too many removed lines in hunk %q", ErrMalformedDiff, currentHunk.Header)
				}
				currentHunk.Lines = append(currentHunk.Lines, model.Line{
					Kind:    model.LineKindRemoved,
					Text:    line[1:],
					OldLine: nextOldLine,
				})
				nextOldLine++
				remainingOldLines--
				continue
			case line == `\ No newline at end of file`:
				continue
			default:
				if !hunkComplete() {
					return model.Diff{}, fmt.Errorf("%w: truncated hunk before %q", ErrMalformedDiff, line)
				}
				flushHunk()
			}
		}

		switch {
		case strings.HasPrefix(line, "diff --git "):
			if err := flushFile(); err != nil {
				return model.Diff{}, err
			}
			oldPath, newPath, err := parseGitHeader(line)
			if err != nil {
				return model.Diff{}, err
			}
			currentFile = &model.File{
				OldPath: oldPath,
				NewPath: newPath,
				Status:  model.FileStatusUnknown,
			}
			currentStarted = false
		case strings.HasPrefix(line, "new file mode "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusAdded
		case strings.HasPrefix(line, "deleted file mode "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusDeleted
		case strings.HasPrefix(line, "rename from "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusRenamed
			pathText, err := parseMetadataPath(strings.TrimPrefix(line, "rename from "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.OldPath = pathText
		case strings.HasPrefix(line, "rename to "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusRenamed
			pathText, err := parseMetadataPath(strings.TrimPrefix(line, "rename to "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.NewPath = pathText
		case strings.HasPrefix(line, "copy from "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusCopied
			pathText, err := parseMetadataPath(strings.TrimPrefix(line, "copy from "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.OldPath = pathText
		case strings.HasPrefix(line, "copy to "):
			currentFile = ensureFile(currentFile)
			currentFile.Status = model.FileStatusCopied
			pathText, err := parseMetadataPath(strings.TrimPrefix(line, "copy to "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.NewPath = pathText
		case strings.HasPrefix(line, "--- "):
			if currentStarted {
				if err := flushFile(); err != nil {
					return model.Diff{}, err
				}
			}
			currentFile = ensureFile(currentFile)
			pathText, err := parsePatchPath(strings.TrimPrefix(line, "--- "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.OldPath = pathText
			currentStarted = true
		case strings.HasPrefix(line, "+++ "):
			currentFile = ensureFile(currentFile)
			pathText, err := parsePatchPath(strings.TrimPrefix(line, "+++ "))
			if err != nil {
				return model.Diff{}, err
			}
			currentFile.NewPath = pathText
			currentStarted = true
		case strings.HasPrefix(line, "@@ "):
			if currentFile == nil {
				return model.Diff{}, fmt.Errorf("%w: hunk without file header", ErrMalformedDiff)
			}
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return model.Diff{}, err
			}
			currentHunk = &hunk
			nextOldLine = hunk.OldStart
			nextNewLine = hunk.NewStart
			remainingOldLines = hunk.OldLines
			remainingNewLines = hunk.NewLines
			currentStarted = true
		}
	}

	if err := flushFile(); err != nil {
		return model.Diff{}, err
	}

	if len(diff.Files) == 0 {
		return model.Diff{}, ErrMalformedDiff
	}

	return diff, nil
}

func ensureFile(file *model.File) *model.File {
	if file != nil {
		return file
	}
	return &model.File{Status: model.FileStatusUnknown}
}

func parseGitHeader(line string) (string, string, error) {
	rest := strings.TrimPrefix(line, "diff --git ")

	if strings.Contains(rest, `"`) {
		oldToken, remaining, err := consumeGitHeaderPath(rest)
		if err != nil {
			return "", "", err
		}
		newToken, remaining, err := consumeGitHeaderPath(remaining)
		if err != nil {
			return "", "", err
		}
		if strings.TrimSpace(remaining) != "" {
			return "", "", fmt.Errorf("%w: invalid git header %q", ErrMalformedDiff, line)
		}

		oldPath, err := parseDiffHeaderPath(oldToken, "a/")
		if err != nil {
			return "", "", err
		}
		newPath, err := parseDiffHeaderPath(newToken, "b/")
		if err != nil {
			return "", "", err
		}
		return oldPath, newPath, nil
	}

	oldToken, remaining, err := consumeGitHeaderPath(rest)
	if err != nil {
		return "", "", err
	}
	newToken, remaining, err := consumeGitHeaderPath(remaining)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(remaining) != "" {
		return "", "", fmt.Errorf("%w: invalid git header %q", ErrMalformedDiff, line)
	}
	oldPath, err := parseDiffHeaderPath(oldToken, "a/")
	if err != nil {
		return "", "", err
	}
	newPath, err := parseDiffHeaderPath(newToken, "b/")
	if err != nil {
		return "", "", err
	}
	return oldPath, newPath, nil
}

func parseDiffHeaderPath(raw, prefix string) (string, error) {
	pathText, err := decodePathText(raw)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(pathText, prefix) {
		pathText = strings.TrimPrefix(pathText, prefix)
	}
	return model.NormalizePath(pathText), nil
}

func parsePatchPath(raw string) (string, error) {
	pathText, err := decodePathText(raw)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(pathText, "a/") || strings.HasPrefix(pathText, "b/") {
		pathText = pathText[2:]
	}
	return model.NormalizePath(pathText), nil
}

func parseMetadataPath(raw string) (string, error) {
	return decodePathText(raw)
}

func decodePathText(raw string) (string, error) {
	pathText := strings.TrimSpace(raw)
	if index := strings.IndexByte(pathText, '\t'); index >= 0 {
		pathText = pathText[:index]
	}
	if strings.HasPrefix(pathText, `"`) {
		unquoted, err := strconv.Unquote(pathText)
		if err != nil {
			return "", fmt.Errorf("%w: invalid quoted path %q", ErrMalformedDiff, raw)
		}
		pathText = unquoted
	}
	return model.NormalizePath(pathText), nil
}

func consumeGitHeaderPath(input string) (string, string, error) {
	trimmed := strings.TrimLeft(input, " ")
	if trimmed == "" {
		return "", "", fmt.Errorf("%w: invalid git header %q", ErrMalformedDiff, input)
	}

	if trimmed[0] != '"' {
		index := strings.IndexByte(trimmed, ' ')
		if index == -1 {
			return trimmed, "", nil
		}
		return trimmed[:index], strings.TrimLeft(trimmed[index+1:], " "), nil
	}

	escaped := false
	for index := 1; index < len(trimmed); index++ {
		switch trimmed[index] {
		case '\\':
			escaped = !escaped
		case '"':
			if !escaped {
				return trimmed[:index+1], strings.TrimLeft(trimmed[index+1:], " "), nil
			}
			escaped = false
		default:
			escaped = false
		}
	}

	return "", "", fmt.Errorf("%w: invalid quoted git header path %q", ErrMalformedDiff, input)
}

func normalizeNewlines(data []byte) []byte {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
}

func splitLines(data []byte) []string {
	rawLines := bytes.Split(data, []byte("\n"))
	if len(rawLines) > 0 && len(rawLines[len(rawLines)-1]) == 0 {
		rawLines = rawLines[:len(rawLines)-1]
	}

	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		lines = append(lines, string(rawLine))
	}
	return lines
}

func parseHunkHeader(line string) (model.Hunk, error) {
	matches := hunkHeaderRE.FindStringSubmatch(line)
	if matches == nil {
		return model.Hunk{}, fmt.Errorf("%w: invalid hunk header %q", ErrMalformedDiff, line)
	}

	oldStart, oldLines := parseHunkRange(matches[1], matches[2])
	newStart, newLines := parseHunkRange(matches[3], matches[4])

	return model.Hunk{
		Header:   line,
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

func parseHunkRange(startText, countText string) (int, int) {
	start := atoi(startText)
	count := 1
	if countText != "" {
		count = atoi(countText)
	}
	return start, count
}

func finalizeFile(file *model.File) {
	if file.Status == model.FileStatusUnknown {
		switch {
		case file.OldPath == "" && file.NewPath != "":
			file.Status = model.FileStatusAdded
		case file.OldPath != "" && file.NewPath == "":
			file.Status = model.FileStatusDeleted
		case file.OldPath != "" && file.NewPath != "" && file.OldPath != file.NewPath:
			file.Status = model.FileStatusRenamed
		default:
			file.Status = model.FileStatusModified
		}
	}

	switch file.Status {
	case model.FileStatusDeleted:
		file.Path = file.OldPath
	default:
		if file.NewPath != "" {
			file.Path = file.NewPath
		} else {
			file.Path = file.OldPath
		}
	}

	file.Path = model.NormalizePath(file.Path)
	file.OldPath = model.NormalizePath(file.OldPath)
	file.NewPath = model.NormalizePath(file.NewPath)
}

func atoi(value string) int {
	var parsed int
	for _, r := range value {
		parsed = parsed*10 + int(r-'0')
	}
	return parsed
}
