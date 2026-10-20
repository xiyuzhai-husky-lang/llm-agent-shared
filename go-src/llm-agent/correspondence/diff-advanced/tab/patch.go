package tab

// The unified patch Git writes for two commits, read beside the name-status listing.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// lineKind is what a hunk line is: unchanged, added by the candidate, or removed by the candidate.
type lineKind int

const (
	lineContext lineKind = iota
	lineInsertion
	lineDeletion
)

// diffLine is one line of one hunk, with the line numbers it carries on each side (zero when absent).
type diffLine struct {
	kind          lineKind
	oldLineNumber uint32
	newLineNumber uint32
	text          string
}

type patchHunk struct {
	headerLine string
	oldStart   uint32
	oldLines   uint32
	newStart   uint32
	newLines   uint32
	lines      []diffLine
	additions  uint32
	deletions  uint32
}

// canonicalHeader is the canonical range of the hunk: the counts always written out, the function context dropped.
func (hunk *patchHunk) canonicalHeader() string {
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.oldStart, hunk.oldLines, hunk.newStart, hunk.newLines)
}

type patchFileSection struct {
	oldHeaderPath    string
	hasOldHeaderPath bool
	newHeaderPath    string
	hasNewHeaderPath bool
	isBinary         bool
	oldEndingNewline bool
	newEndingNewline bool
	hunks            []patchHunk
}

var extendedHeaderPrefixes = []string{"index ", "old mode ", "new mode ", "similarity index ", "dissimilarity index ", "Submodule "}

// parseUnifiedPatchLines reads the sections of a patch; a section's hunk lines are kept only when `retainLines` says so
// for the section's index.
func parseUnifiedPatchLines(lines []string, retainLines func(int) bool) ([]patchFileSection, error) {
	var sections []patchFileSection
	for position := 0; position < len(lines); {
		line := lines[position]
		position++
		retainIndex := len(sections) - 1
		if retainIndex < 0 {
			retainIndex = 0
		}
		retain := retainLines(retainIndex)
		if strings.HasPrefix(line, "diff --git ") {
			sections = append(sections, patchFileSection{oldEndingNewline: true, newEndingNewline: true})
			continue
		}
		if len(sections) == 0 {
			return nil, fmt.Errorf("the patch begins with %q instead of a `diff --git` line", line)
		}
		section := &sections[len(sections)-1]
		switch {
		case strings.HasPrefix(line, "@@ "):
			hunk, next, err := parseHunk(line, lines, position, section, retain)
			if err != nil {
				return nil, err
			}
			position = next
			section.hunks = append(section.hunks, hunk)
		case strings.HasPrefix(line, "--- "):
			path, present, err := sideHeaderPath(line, "--- ", "a/")
			if err != nil {
				return nil, err
			}
			section.oldHeaderPath, section.hasOldHeaderPath = path, present
		case strings.HasPrefix(line, "+++ "):
			path, present, err := sideHeaderPath(line, "+++ ", "b/")
			if err != nil {
				return nil, err
			}
			section.newHeaderPath, section.hasNewHeaderPath = path, present
		case strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch":
			section.isBinary = true
		case strings.HasPrefix(line, "new file mode "), strings.HasPrefix(line, "deleted file mode "):
		case strings.HasPrefix(line, "rename from "), strings.HasPrefix(line, "rename to "),
			strings.HasPrefix(line, "copy from "), strings.HasPrefix(line, "copy to "):
			rest := line[strings.Index(line, " ")+1:]
			rest = rest[strings.Index(rest, " ")+1:]
			if _, err := decodePatchHeaderPath(rest); err != nil {
				return nil, err
			}
		default:
			known := false
			for _, prefix := range extendedHeaderPrefixes {
				if strings.HasPrefix(line, prefix) {
					known = true
				}
			}
			if !known {
				return nil, fmt.Errorf("the patch line %q is not a line this parser knows", line)
			}
		}
	}
	return sections, nil
}

// sideHeaderPath reads `--- a/<path>` or `+++ b/<path>`; `/dev/null` names an absent side.
func sideHeaderPath(line, marker, prefix string) (string, bool, error) {
	rest := strings.TrimPrefix(line, marker)
	rest = strings.TrimSuffix(rest, "\t")
	path, err := decodePatchHeaderPath(rest)
	if err != nil {
		return "", false, err
	}
	if path == "/dev/null" {
		return "", false, nil
	}
	if !strings.HasPrefix(path, prefix) {
		return "", false, fmt.Errorf("the patch file header %q is not `diff --git a/<path> b/<path>`", line)
	}
	return strings.TrimPrefix(path, prefix), true, nil
}

func parseHunk(headerLine string, lines []string, position int, section *patchFileSection, retain bool) (patchHunk, int, error) {
	oldStart, oldLines, newStart, newLines, err := parseHunkHeader(headerLine)
	if err != nil {
		return patchHunk{}, 0, err
	}
	hunk := patchHunk{headerLine: headerLine, oldStart: oldStart, oldLines: oldLines, newStart: newStart, newLines: newLines}
	remainingOld, remainingNew := oldLines, newLines
	oldNumber, newNumber := oldStart, newStart
	for remainingOld > 0 || remainingNew > 0 {
		if position >= len(lines) {
			return patchHunk{}, 0, fmt.Errorf("the hunk %q ends before its declared %d old and %d new lines are read", headerLine, oldLines, newLines)
		}
		lineIndex := position
		line := lines[position]
		position++
		var kind lineKind
		switch {
		case strings.HasPrefix(line, " "):
			kind = lineContext
		case strings.HasPrefix(line, "+"):
			kind = lineInsertion
		case strings.HasPrefix(line, "-"):
			kind = lineDeletion
		default:
			return patchHunk{}, 0, fmt.Errorf("at patch line %d, the hunk %q carries %q, which is neither context (` `), an insertion (`+`), a deletion (`-`) nor a missing-newline marker (`\\`)", lineIndex+1, headerLine, line)
		}
		text := line[1:]
		consumesOld := kind != lineInsertion
		consumesNew := kind != lineDeletion
		if consumesOld && remainingOld == 0 {
			return patchHunk{}, 0, fmt.Errorf("the hunk %q carries more old lines than its header declares, starting at %q", headerLine, line)
		}
		if consumesNew && remainingNew == 0 {
			return patchHunk{}, 0, fmt.Errorf("the hunk %q carries more new lines than its header declares, starting at %q", headerLine, line)
		}
		var oldLine, newLine uint32
		if consumesOld {
			oldLine = oldNumber
			oldNumber++
			remainingOld--
		}
		if consumesNew {
			newLine = newNumber
			newNumber++
			remainingNew--
		}
		switch kind {
		case lineInsertion:
			hunk.additions++
		case lineDeletion:
			hunk.deletions++
		}
		if retain {
			hunk.lines = append(hunk.lines, diffLine{kind: kind, oldLineNumber: oldLine, newLineNumber: newLine, text: text})
		}
		// Git writes `\ No newline at end of file` straight after the line that lacks the newline.
		if position < len(lines) && strings.HasPrefix(lines[position], "\\") {
			marker := lines[position]
			position++
			if marker != "\\ No newline at end of file" {
				return patchHunk{}, 0, fmt.Errorf("at patch line %d, the hunk %q carries %q, which is neither context (` `), an insertion (`+`), a deletion (`-`) nor a missing-newline marker (`\\`)", position, headerLine, marker)
			}
			if consumesOld {
				section.oldEndingNewline = false
			}
			if consumesNew {
				section.newEndingNewline = false
			}
		}
	}
	return hunk, position, nil
}

// parseHunkHeader reads `@@ -<start>[,<lines>] +<start>[,<lines>] @@[ <function context>]`.
func parseHunkHeader(line string) (uint32, uint32, uint32, uint32, error) {
	unreadable := fmt.Errorf("the hunk header %q is not `@@ -<start>[,<lines>] +<start>[,<lines>] @@`", line)
	rest, found := strings.CutPrefix(line, "@@ -")
	if !found {
		return 0, 0, 0, 0, unreadable
	}
	oldRange, rest, found := strings.Cut(rest, " +")
	if !found {
		return 0, 0, 0, 0, unreadable
	}
	newRange, _, found := strings.Cut(rest, " @@")
	if !found {
		return 0, 0, 0, 0, unreadable
	}
	oldStart, oldLines, okOld := parseHunkRange(oldRange)
	newStart, newLines, okNew := parseHunkRange(newRange)
	if !okOld || !okNew {
		return 0, 0, 0, 0, unreadable
	}
	return oldStart, oldLines, newStart, newLines, nil
}

func parseHunkRange(text string) (uint32, uint32, bool) {
	parse := func(digits string) (uint32, bool) {
		value, err := strconv.ParseUint(digits, 10, 32)
		return uint32(value), err == nil && digits != "" && digits[0] != '+'
	}
	start, lines, hasLines := strings.Cut(text, ",")
	startValue, ok := parse(start)
	if !ok {
		return 0, 0, false
	}
	if !hasLines {
		return startValue, 1, true
	}
	linesValue, ok := parse(lines)
	return startValue, linesValue, ok
}

// decodePatchHeaderPath decodes the path Git writes in a patch header: with `core.quotePath=true` a path carrying
// non-ASCII or control bytes is C-quoted, its UTF-8 bytes escaped as three-digit octal.
func decodePatchHeaderPath(text string) (string, error) {
	malformed := func(offset int, escape string) error {
		return fmt.Errorf("quoted path %q: invalid escape %q at offset %d", text, escape, offset)
	}
	if !strings.HasPrefix(text, `"`) && !strings.HasSuffix(text, `"`) {
		for offset, character := range text {
			if character == '"' || character == '\\' {
				return "", malformed(offset, fmt.Sprintf("unquoted character %q", character))
			}
		}
		return text, nil
	}
	if len(text) < 2 || !strings.HasPrefix(text, `"`) || !strings.HasSuffix(text, `"`) {
		return "", malformed(0, "unpaired quote")
	}
	body := []rune(text[1 : len(text)-1])
	var decoded []byte
	for index := 0; index < len(body); {
		if body[index] != '\\' {
			if body[index] == '"' {
				return "", malformed(index+1, "unescaped quote")
			}
			decoded = utf8.AppendRune(decoded, body[index])
			index++
			continue
		}
		if index+4 <= len(body) {
			value, octal := 0, true
			for _, digit := range body[index+1 : index+4] {
				if digit < '0' || digit > '7' {
					octal = false
					break
				}
				value = value*8 + int(digit-'0')
			}
			if octal && value <= 255 {
				decoded = append(decoded, byte(value))
				index += 4
				continue
			}
		}
		if index+1 >= len(body) {
			return "", malformed(index+1, "trailing backslash")
		}
		escapes := map[rune]byte{'a': 7, 'b': 8, 'v': 11, 'f': 12, '\\': '\\', '"': '"', 't': '\t', 'n': '\n', 'r': '\r'}
		value, known := escapes[body[index+1]]
		if !known {
			return "", malformed(index+1, `\`+string(body[index+1]))
		}
		decoded = append(decoded, value)
		index += 2
	}
	if !utf8.Valid(decoded) {
		return "", fmt.Errorf("the quoted path %q does not decode to UTF-8", text)
	}
	return string(decoded), nil
}
