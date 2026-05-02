package diffparse

import (
	"bufio"
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

	scanner := bufio.NewScanner(bytes.NewReader(normalized))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var diff model.Diff
	var currentFile *model.File
	var currentHunk *model.Hunk
	var nextOldLine int
	var nextNewLine int
	var remainingOldLines int
	var remainingNewLines int
	var currentStarted bool

	flushHunk := func() {
		if currentFile == nil || currentHunk == nil {
			return
		}
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		currentHunk = nil
		remainingOldLines = 0
		remainingNewLines = 0
	}

	flushFile := func() {
		if currentFile == nil {
			return
		}
		flushHunk()
		finalizeFile(currentFile)
		if currentFile.Path != "" || len(currentFile.Hunks) > 0 {
			diff.Files = append(diff.Files, *currentFile)
		}
		currentFile = nil
		currentStarted = false
	}

	for scanner.Scan() {
		line := scanner.Text()

		if currentHunk != nil && remainingOldLines == 0 && remainingNewLines == 0 {
			flushHunk()
		}

		if currentHunk != nil {
			switch {
			case strings.HasPrefix(line, "@@ "):
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
				currentHunk.Lines = append(currentHunk.Lines, model.Line{
					Kind:    model.LineKindAdded,
					Text:    line[1:],
					NewLine: nextNewLine,
				})
				nextNewLine++
				remainingNewLines--
				continue
			case strings.HasPrefix(line, "-"):
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
				flushHunk()
			}
		}

		switch {
		case strings.HasPrefix(line, "diff --git "):
			flushFile()
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
				flushFile()
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

	if err := scanner.Err(); err != nil {
		return model.Diff{}, fmt.Errorf("scan diff: %w", err)
	}

	flushFile()

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

	separator := strings.Index(rest, " b/")
	if !strings.HasPrefix(rest, "a/") || separator == -1 {
		return "", "", fmt.Errorf("%w: invalid git header %q", ErrMalformedDiff, line)
	}

	oldPath, err := parseDiffHeaderPath(rest[:separator], "a/")
	if err != nil {
		return "", "", err
	}
	newPath, err := parseDiffHeaderPath(rest[separator+1:], "b/")
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
	if !strings.HasPrefix(pathText, prefix) {
		return "", fmt.Errorf("%w: invalid git header path %q", ErrMalformedDiff, raw)
	}
	return model.NormalizePath(strings.TrimPrefix(pathText, prefix)), nil
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
