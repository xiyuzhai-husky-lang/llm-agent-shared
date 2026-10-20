// Package notebook reads the shared source-range outline. Discussion is prose, not source evidence.
package notebook

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Reference struct {
	Path     string `json:"path"`
	Start    int    `json:"start_line"`
	End      int    `json:"end_line"`
	Revision string `json:"-"`
}
type Line struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
	Quoted bool   `json:"quoted"`
}
type Item struct {
	Key        string    `json:"key"`
	Title      string    `json:"title"`
	Source     Reference `json:"source"`
	Discussion string    `json:"discussion"`
	RangeLabel string    `json:"range_label"`
	Lines      []Line    `json:"lines"`
}
type Group struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Items []Item `json:"items"`
}
type Notebook struct {
	Path      string  `json:"path"`
	Groups    []Group `json:"groups"`
	ItemCount int     `json:"item_count"`
}

var heading = regexp.MustCompile(`^\s{0,3}#{1,6}(?:\s+(.*?)\s*#*\s*|\s*)$`)
var discussion = regexp.MustCompile(`^\s{2,}>\s?(.*)$`)
var bullet = regexp.MustCompile(`^\s{0,3}[-*+]\s+\[([^]]+)\]\(([^)]+)\)\s*$`)
var reference = regexp.MustCompile(`^([^#]+)#L([1-9][0-9]*)(?:-L([1-9][0-9]*))?(?:@([0-9a-f]{40,64}))?$`)

func plain(text string) string {
	return strings.TrimSpace(strings.NewReplacer("*", "", "_", "", "`", "").Replace(text))
}
func Parse(path, text string) (*Notebook, error) {
	result := &Notebook{Path: path, Groups: []Group{}}
	group := Group{Title: "Sources", Items: []Item{}}
	flush := func() {
		if len(group.Items) > 0 {
			group.Key = fmt.Sprintf("group-%d", len(result.Groups))
			for i := range group.Items {
				group.Items[i].Discussion = strings.TrimSpace(group.Items[i].Discussion)
			}
			result.Groups = append(result.Groups, group)
		}
	}
	collecting := false
	for index, line := range strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n") {
		if match := heading.FindStringSubmatch(line); match != nil {
			title := plain(match[1])
			if title == "" {
				return nil, fmt.Errorf("%s:%d: empty heading", path, index+1)
			}
			flush()
			group = Group{Title: title, Items: []Item{}}
			collecting = false
			continue
		}
		if match := discussion.FindStringSubmatch(line); match != nil && collecting {
			group.Items[len(group.Items)-1].Discussion += match[1] + "\n"
			continue
		}
		match := bullet.FindStringSubmatch(line)
		if match == nil {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 1 && strings.ContainsAny(trimmed[:1], "-*+") && strings.HasPrefix(strings.TrimSpace(trimmed[1:]), "[") {
				return nil, fmt.Errorf("%s:%d: malformed source item", path, index+1)
			}
			if trimmed != "" {
				collecting = false
			}
			continue
		}
		target := strings.TrimSpace(match[2])
		if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
			target = target[1 : len(target)-1]
		}
		ref := reference.FindStringSubmatch(target)
		if ref == nil || !filepath.IsLocal(ref[1]) || strings.ContainsAny(ref[1], "\\\x00\r\n") || strings.Contains(ref[1], "../") {
			return nil, fmt.Errorf("%s:%d: invalid source reference %q", path, index+1, target)
		}
		start, err := strconv.ParseUint(ref[2], 10, 32)
		if err != nil {
			return nil, err
		}
		end := start
		if ref[3] != "" {
			end, err = strconv.ParseUint(ref[3], 10, 32)
			if err != nil {
				return nil, err
			}
		}
		if end < start {
			return nil, fmt.Errorf("%s:%d: reversed source range", path, index+1)
		}
		label := fmt.Sprintf("Lines %d–%d", start, end)
		if start == end {
			label = fmt.Sprintf("Line %d", start)
		}
		group.Items = append(group.Items, Item{Key: fmt.Sprintf("item-%d-%d", len(result.Groups), len(group.Items)), Title: plain(match[1]), Source: Reference{ref[1], int(start), int(end), ref[4]}, RangeLabel: label, Lines: []Line{}})
		result.ItemCount++
		collecting = true
	}
	flush()
	return result, nil
}
func (n *Notebook) Bind(revision string, read func(string, string) (string, error)) error {
	sources := map[string]string{}
	for g := range n.Groups {
		for i := range n.Groups[g].Items {
			item := &n.Groups[g].Items[i]
			oid := item.Source.Revision
			if oid == "" {
				oid = revision
			}
			key := oid + ":" + item.Source.Path
			source, ok := sources[key]
			if !ok {
				var err error
				source, err = read(oid, item.Source.Path)
				if err != nil {
					return err
				}
				sources[key] = source
			}
			lines := []string{}
			if source != "" {
				lines = strings.Split(strings.TrimSuffix(strings.ReplaceAll(source, "\r\n", "\n"), "\n"), "\n")
			}
			if item.Source.End > len(lines) {
				return fmt.Errorf("%s: %s source range exceeds %d lines", n.Path, item.Key, len(lines))
			}
			for j, text := range lines {
				if text == "" {
					text = "\u00a0"
				}
				item.Lines = append(item.Lines, Line{j + 1, text, j+1 >= item.Source.Start && j+1 <= item.Source.End})
			}
		}
	}
	return nil
}
