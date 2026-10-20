package highlight

import (
	"container/heap"
	"context"
	"fmt"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"html"
	"sort"
	"strings"
	"unicode/utf8"
)

// document retains parsed captures and source only during one delivery.
type document struct {
	text  string
	spans []span
	lines []int
}

func parse(ctx context.Context, language, text string) (*document, error) {
	return parseWith(ctx, language, text, grammars, captureClass)
}
func parseWith(ctx context.Context, language, text string, queries map[string]*grammar, classify func(string) string) (*document, error) {
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("source is not UTF-8")
	}
	d := &document{text: text, lines: []int{0}}
	for i, c := range text {
		if c == '\n' {
			d.lines = append(d.lines, i+1)
		}
	}
	if language == "" || text == "" {
		return d, nil
	}
	names := []string{language}
	if language == "markdown" {
		names = append(names, "markdown_inline")
	}
	for _, name := range names {
		g := queries[name]
		if g == nil {
			return nil, fmt.Errorf("no grammar for %s", name)
		}
		query, err := g.compile(name)
		if err != nil {
			return nil, fmt.Errorf("%s query: %w", name, err)
		}
		parser := sitter.NewParser()
		defer parser.Close()
		if err := parser.SetLanguage(sitter.NewLanguage(g.language())); err != nil {
			return nil, fmt.Errorf("incompatible %s grammar", name)
		}
		source := []byte(text)
		tree := parser.ParseWithOptions(func(offset int, _ sitter.Point) []byte { return source[offset:] }, nil, &sitter.ParseOptions{ProgressCallback: func(sitter.ParseState) bool { return ctx.Err() != nil }})
		if tree == nil {
			return nil, fmt.Errorf("%s parse cancelled: %v", name, ctx.Err())
		}
		defer tree.Close()
		cursor := sitter.NewQueryCursor()
		defer cursor.Close()
		captures := cursor.Captures(query, tree.RootNode(), []byte(text))
		for {
			match, index := captures.Next()
			if match == nil {
				break
			}
			capture := match.Captures[index]
			class := classify(query.CaptureNames()[capture.Index])
			if class == "" {
				continue
			}
			d.spans = append(d.spans, span{int(capture.Node.StartByte()), int(capture.Node.EndByte()), uint(match.PatternIndex), class})
			if len(d.spans) > 1000000 {
				return nil, fmt.Errorf("highlight capture limit exceeded")
			}
			if len(d.spans)%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
		}
		if cursor.DidExceedMatchLimit() {
			return nil, fmt.Errorf("%s query match limit exceeded", name)
		}
	}
	sort.SliceStable(d.spans, func(i, j int) bool { return d.spans[i].start < d.spans[j].start })
	return d, nil
}

func captureClass(name string) string {
	head, _, _ := strings.Cut(name, ".")
	switch head {
	case "keyword", "conditional", "repeat", "exception", "include":
		return "keyword"
	case "function", "method", "constructor":
		return "function"
	case "type", "namespace", "module":
		return "type"
	case "string", "character":
		return "string"
	case "number", "float", "boolean", "constant":
		return "constant"
	case "comment":
		return "comment"
	case "operator", "punctuation":
		return "punctuation"
	case "variable", "parameter", "property", "field", "attribute":
		return "variable"
	case "tag", "label":
		return "tag"
	case "text", "markup":
		return "markup"
	}
	return ""
}

func (d *document) render(line int, text string) (string, error) {
	if text == "" {
		return "", nil
	}
	if line < 1 || line > len(d.lines) {
		return "", fmt.Errorf("hunk line %d is outside file", line)
	}
	start := d.lines[line-1]
	end := start + len(text)
	if end > len(d.text) || d.text[start:end] != text {
		return "", fmt.Errorf("hunk does not match pinned source at line %d", line)
	}
	points := []int{start, end}
	spans := []span{}
	for _, s := range d.spans {
		if s.start >= end {
			break
		}
		if s.end <= start {
			continue
		}
		spans = append(spans, s)
		points = append(points, max(start, s.start), min(end, s.end))
	}
	sort.Ints(points)
	active := &activeSpans{}
	next := 0
	var out strings.Builder
	for i := 0; i+1 < len(points); i++ {
		pos, until := points[i], points[i+1]
		if pos == until {
			continue
		}
		for next < len(spans) && spans[next].start <= pos {
			heap.Push(active, spans[next])
			next++
		}
		for active.Len() > 0 && (*active)[0].end <= pos {
			heap.Pop(active)
		}
		value := html.EscapeString(d.text[pos:until])
		if active.Len() > 0 {
			out.WriteString(`<span class="syntax-` + (*active)[0].class + `">`)
			out.WriteString(value)
			out.WriteString(`</span>`)
		} else {
			out.WriteString(value)
		}
	}
	return out.String(), nil
}
