package tab

// What makes a changed file and its hunks MUST SIGN: the instance's `must-sign-patterns.txt`, which marks whole files by
// glob, and its `must-sign-additions.json`, which marks parts of files by candidate-side line ranges.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

const (
	mustSignPatternsFile  = "must-sign-patterns.txt"
	mustSignAdditionsFile = "must-sign-additions.json"
)

// mustSignPattern is one line of `must-sign-patterns.txt`: a glob, or with a leading `!` an exclusion. `?` matches one
// character inside a path segment, `*` any run inside a segment, and `**` spans directories, `**/` also matching none.
type mustSignPattern struct {
	pattern  string
	excludes bool
	matcher  *regexp.Regexp
}

func parseMustSignPattern(line string) (mustSignPattern, error) {
	excludes := strings.HasPrefix(line, "!")
	pattern := strings.TrimPrefix(line, "!")
	matcher, err := regexp.Compile(globMatcherSource(pattern))
	if err != nil {
		return mustSignPattern{}, fmt.Errorf("must-sign pattern %q does not compile to a matcher: %v", pattern, err)
	}
	return mustSignPattern{pattern: pattern, excludes: excludes, matcher: matcher}, nil
}

func globMatcherSource(pattern string) string {
	characters := []rune(pattern)
	var source strings.Builder
	source.WriteString("^")
	for index := 0; index < len(characters); index++ {
		switch characters[index] {
		case '*':
			if index+1 < len(characters) && characters[index+1] == '*' {
				index++
				if index+1 < len(characters) && characters[index+1] == '/' {
					index++
					source.WriteString("(?:.*/)?")
				} else {
					source.WriteString(".*")
				}
			} else {
				source.WriteString("[^/]*")
			}
		case '?':
			source.WriteString("[^/]")
		default:
			if strings.ContainsRune(".+^${}()|[]\\", characters[index]) {
				source.WriteByte('\\')
			}
			source.WriteRune(characters[index])
		}
	}
	source.WriteString("$")
	return source.String()
}

// pathIsMustSignUnderPatterns is whether some glob matches the path and no exclusion does.
func pathIsMustSignUnderPatterns(path string, patterns []mustSignPattern) bool {
	included := false
	for _, pattern := range patterns {
		if pattern.matcher.MatchString(path) {
			if pattern.excludes {
				return false
			}
			included = true
		}
	}
	return included
}

func mustSignPatternsFromText(text string) ([]mustSignPattern, error) {
	var patterns []mustSignPattern
	for _, line := range strLines(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, err := parseMustSignPattern(line)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// mustSignLineRange is a run of candidate-side lines, one-based and inclusive at both ends.
type mustSignLineRange struct {
	Start uint32 `json:"start"`
	End   uint32 `json:"end"`
}

// mustSignAddition is one part of one changed file the instance marks MUST SIGN: the candidate-side line ranges whose
// hunks must be signed, and the one-sentence reason the Diff tab shows beside the file.
type mustSignAddition struct {
	Path   string              `json:"path"`
	Lines  []mustSignLineRange `json:"lines"`
	Reason string              `json:"reason"`
}

type mustSignAdditionsDocument struct {
	Additions []mustSignAddition `json:"additions"`
}

// readMustSignAdditions reads `must-sign-additions.json`, which every instance has: a missing file, a body that is
// not exactly `{"additions": [{"path", "lines": [{"start", "end"}], "reason"}]}`, or an empty list is an error.
func readMustSignAdditions(file string) ([]mustSignAddition, error) {
	text, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read the must-sign additions file %s: %v", file, err)
	}
	invalid := func(reason string) error {
		return fmt.Errorf(`the must-sign additions file %s is not {"additions": [{"path", "lines": [{"start", "end"}], "reason"}]}: %s`, file, reason)
	}
	if err := requireFields(text, []string{"additions"}); err != nil {
		return nil, invalid(err.Error())
	}
	var document mustSignAdditionsDocument
	if err := decodeStrict(text, &document); err != nil {
		return nil, invalid(err.Error())
	}
	var raw struct {
		Additions []json.RawMessage `json:"additions"`
	}
	if err := json.Unmarshal(text, &raw); err != nil {
		return nil, invalid(err.Error())
	}
	for index, addition := range raw.Additions {
		if err := requireFields(addition, []string{"path", "lines", "reason"}); err != nil {
			return nil, invalid(fmt.Sprintf("additions[%d]: %v", index, err))
		}
		var rawLines struct {
			Lines []json.RawMessage `json:"lines"`
		}
		if err := json.Unmarshal(addition, &rawLines); err != nil {
			return nil, invalid(err.Error())
		}
		for lineIndex, lineRange := range rawLines.Lines {
			if err := requireFields(lineRange, []string{"start", "end"}); err != nil {
				return nil, invalid(fmt.Sprintf("additions[%d].lines[%d]: %v", index, lineIndex, err))
			}
		}
	}
	if len(document.Additions) == 0 {
		return nil, fmt.Errorf("the must-sign additions file %s lists no addition: every advanced diff names at least one part to sign", file)
	}
	return document.Additions, nil
}

// mustSignPolicy is the instance's patterns together with its additions: what decides whether a changed file and each
// of its hunks is MUST SIGN.
type mustSignPolicy struct {
	patterns  []mustSignPattern
	additions []mustSignAddition
}

func (policy *mustSignPolicy) addition(path string) *mustSignAddition {
	for index := range policy.additions {
		if policy.additions[index].Path == path {
			return &policy.additions[index]
		}
	}
	return nil
}

// pathIsMustSign is whether the file is listed under MUST SIGN: the patterns mark it whole, or an addition names part
// of it.
func (policy *mustSignPolicy) pathIsMustSign(path string) bool {
	return pathIsMustSignUnderPatterns(path, policy.patterns) || policy.addition(path) != nil
}

// hunkNewSideSpan is the candidate-side lines a hunk covers; a hunk that only deletes covers the line its deletion
// follows, line one when the deletion is at the top of the file.
func hunkNewSideSpan(newStart, newLines uint32) (uint32, uint32) {
	if newLines == 0 {
		return max(newStart, 1), max(newStart, 1)
	}
	return newStart, newStart + newLines - 1
}

func (lineRange mustSignLineRange) overlaps(start, end uint32) bool {
	return lineRange.Start <= end && start <= lineRange.End
}

// hunkIsMustSign is whether a hunk of `path` is MUST SIGN: every hunk of a file the patterns mark, and a hunk of an
// added file whose candidate-side lines meet one of the addition's ranges.
func (policy *mustSignPolicy) hunkIsMustSign(path string, newStart, newLines uint32) bool {
	if pathIsMustSignUnderPatterns(path, policy.patterns) {
		return true
	}
	addition := policy.addition(path)
	if addition == nil {
		return false
	}
	start, end := hunkNewSideSpan(newStart, newLines)
	for _, lineRange := range addition.Lines {
		if lineRange.overlaps(start, end) {
			return true
		}
	}
	return false
}

// additionProblems is every reason the additions are malformed for the range: a path the Diff does not list, a path the
// patterns already mark, a path listed twice, an empty or multi-line reason, no line range, a range that is not a run
// of lines of the candidate-side file, and a range no hunk of the file touches.
func additionProblems(policy *mustSignPolicy, files []unnumberedFile, newSideLineCount func(path string) (int, error)) ([]string, error) {
	var problems []string
	for index, addition := range policy.additions {
		path := addition.Path
		fileIndex := slices.IndexFunc(files, func(file unnumberedFile) bool { return file.path == path })
		if fileIndex < 0 {
			problems = append(problems, fmt.Sprintf("%q is not a changed file of the candidate's Diff", path))
		}
		if pathIsMustSignUnderPatterns(path, policy.patterns) {
			problems = append(problems, fmt.Sprintf("%q is already MUST SIGN under the project's must-sign patterns", path))
		}
		if slices.ContainsFunc(policy.additions[:index], func(earlier mustSignAddition) bool { return earlier.Path == path }) {
			problems = append(problems, fmt.Sprintf("%q is listed more than once", path))
		}
		if strings.TrimSpace(addition.Reason) == "" {
			problems = append(problems, fmt.Sprintf("%q has an empty reason", path))
		} else if strings.Contains(addition.Reason, "\n") {
			problems = append(problems, fmt.Sprintf("%q has a reason of more than one line", path))
		}
		if len(addition.Lines) == 0 {
			problems = append(problems, fmt.Sprintf("%q names no line range", path))
		}
		if fileIndex < 0 {
			continue
		}
		file := files[fileIndex]
		if file.changeKind == changeDeleted {
			problems = append(problems, fmt.Sprintf("%q is deleted by the range, so it has no candidate-side lines to name", path))
			continue
		}
		lineCount, err := newSideLineCount(file.newPath)
		if err != nil {
			return nil, err
		}
		for _, lineRange := range addition.Lines {
			if lineRange.Start < 1 || lineRange.End < lineRange.Start || int(lineRange.End) > lineCount {
				problems = append(problems, fmt.Sprintf("%q: lines %d-%d are not a run of the file's %d candidate-side lines",
					path, lineRange.Start, lineRange.End, lineCount))
				continue
			}
			touched := slices.ContainsFunc(file.hunks, func(hunk hunkFields) bool {
				return lineRange.overlaps(hunkNewSideSpan(hunk.newStart, hunk.newLines))
			})
			if !touched {
				problems = append(problems, fmt.Sprintf("%q: lines %d-%d lie in no hunk of the range", path, lineRange.Start, lineRange.End))
			}
		}
	}
	return problems, nil
}

// decodeStrict decodes JSON that carries exactly one value and no field the target does not declare.
func decodeStrict(text []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("trailing data after the JSON value")
	}
	return nil
}

// requireFields fails unless the JSON object carries every one of `fields`, none of them null.
func requireFields(text []byte, fields []string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(text, &object); err != nil {
		return err
	}
	if object == nil {
		return fmt.Errorf("expected an object, found null")
	}
	for _, field := range fields {
		value, present := object[field]
		if !present {
			return fmt.Errorf("missing field `%s`", field)
		}
		if string(value) == "null" {
			return fmt.Errorf("field `%s` is null", field)
		}
	}
	return nil
}

// strLines splits a text into lines the way a line iterator does: `\n` ends a line, a `\r` before it is dropped, and
// a final newline does not open an empty last line.
func strLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		lines = lines[:len(lines)-1]
	}
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}
