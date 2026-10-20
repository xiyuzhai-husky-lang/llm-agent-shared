package reviewer

import (
	"fmt"
	"html"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"strings"
)

func doubtDocument(d api.Doubt) any {
	if d.Document != nil && d.Document.HTML != "" {
		return object{"rendered_html": d.Document.HTML}
	}
	return object{"rendered_html": "<div class=\"ky-document\"><p>" + html.EscapeString(d.Text) + "</p></div>"}
}
func (c *Candidate) finding(index int, d api.Doubt, r service.ReviewRevision) object {
	locations := []object{}
	anchors := []object{}
	sources := d.Anchors
	if len(sources) == 0 && d.Path != "" {
		sources = []api.Anchor{{Path: d.Path, Hunk: d.Hunk, Header: d.Header}}
	}
	for _, a := range sources {
		label := a.Path
		if a.Header != "" {
			label += " " + a.Header
		}
		line := 1
		for _, h := range c.Material.Hunks {
			if h.ID == a.Hunk {
				line = max(1, h.NewStart)
				break
			}
		}
		locations = append(locations, object{"label": label, "path": a.Path, "range": nullable(a.Header), "line": line, "end_line": nil})
		anchors = append(anchors, object{"path": a.Path, "range": nullable(a.Header)})
	}
	return object{"index": index + 1, "identifier": d.ID, "ruling": nullable(r.Rulings[d.ID]), "anchors": anchors, "display": object{"document": doubtDocument(d), "locations": locations, "references": []any{}}}
}
func (c *Candidate) hunkFindings(h api.Hunk, r service.ReviewRevision) []object {
	result := []object{}
	for i, d := range c.Material.Doubts {
		matches := d.Hunk == h.ID
		for _, a := range d.Anchors {
			matches = matches || a.Hunk == h.ID
		}
		if matches {
			result = append(result, c.finding(i, d, r))
		}
	}
	return result
}
func (c *Candidate) doubts(r service.ReviewRevision) object {
	findings, published, displays := []object{}, []object{}, []object{}
	for i, d := range c.Material.Doubts {
		n := fmt.Sprint(i + 1)
		findings = append(findings, c.finding(i, d, r))
		published = append(published, object{"number": n, "conviction": nil, "decision": nullable(r.Rulings[d.ID]), "worker": object{"disposition": d.Worker, "reason": d.Reason}})
		mark := ""
		if d.Worker == "agreed" {
			mark = "✓"
		} else if d.Worker == "disagreed" {
			mark = "✗"
		}
		displays = append(displays, object{"number": n, "doctrines": []any{}, "worker_disposition_mark": mark, "worker_answer": []object{{"text": d.Reason, "code": false}}})
	}
	return object{"heading": fmt.Sprintf("Doubts %d/%d", len(r.Rulings), len(c.Material.Doubts)), "candidate": c.Subject, "findings": findings, "published": object{"path": "Delivered candidate", "doubts": published}, "rule_conflicts": c.ruleConflicts(), "cited_sources": []any{}, "doubt_displays": displays, "link_targets": []any{}}
}

func (c *Candidate) wideFindings(file string, r service.ReviewRevision) []object {
	findings := []object{}
	for i, d := range c.Material.Doubts {
		match := len(d.Anchors) == 0 && d.Hunk == "" && d.Path == file
		for _, a := range d.Anchors {
			match = match || (a.Hunk == "" && a.Path == file)
		}
		if match {
			findings = append(findings, c.finding(i, d, r))
		}
	}
	return findings
}
func (c *Candidate) ruleConflicts() object {
	items := []object{}
	for _, raw := range c.Material.Documents {
		if raw.Kind != "rule-conflict" {
			continue
		}
		doc := c.Documents[raw.ID]
		text := strings.TrimSpace(doc.Text)
		if index := strings.Index(text, "Rule conflicts:"); index >= 0 {
			text = strings.TrimSpace(text[index+len("Rule conflicts:"):])
		}
		if strings.EqualFold(text, "none") || text == "" {
			continue
		}
		markup := doc.HTML
		if markup == "" {
			markup = "<div class=\"ky-document\"><pre>" + html.EscapeString(text) + "</pre></div>"
		}
		items = append(items, object{"number": fmt.Sprint(len(items) + 1), "document": object{"rendered_html": markup}, "references": []any{}})
	}
	return object{"heading": fmt.Sprintf("Rule conflicts · %d", len(items)), "items": items}
}
