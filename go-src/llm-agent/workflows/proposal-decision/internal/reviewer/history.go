package reviewer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// renderHistory restores presentation from the immutable archival documents already
// delivered with a candidate. It never imports signatures or replays verdicts.
func (c *Candidate) renderHistory(ctx context.Context, script, repository string, historyCandidates map[string]bool) error {
	if script == "" {
		return nil
	}
	documents := map[string]json.RawMessage{}
	byText := map[string]string{}
	for _, doc := range c.Material.Documents {
		if doc.ID != "historical-proposal-doubts" {
			continue
		}
		chunks := regexp.MustCompile(`(?m)^[0-9a-f]{40,64}\.json\s*\n`).Split(doc.Text, -1)
		for _, chunk := range chunks {
			var record struct {
				Doubts []struct {
					Text     string          `json:"text"`
					Document json.RawMessage `json:"document"`
				} `json:"doubts"`
			}
			if json.Unmarshal([]byte(strings.TrimSpace(chunk)), &record) != nil {
				continue
			}
			for _, d := range record.Doubts {
				if len(d.Document) == 0 {
					continue
				}
				id := fmt.Sprint(len(documents))
				documents[id] = d.Document
				byText[strings.Join(strings.Fields(d.Text), " ")] = id
			}
		}
	}
	wanted := map[string]json.RawMessage{}
	for id, doc := range c.Documents {
		if doc.HTML != "" {
			continue
		}
		var wire map[string]json.RawMessage
		if json.Unmarshal([]byte(doc.Text), &wire) == nil && wire["arena"] != nil {
			wanted["surface:"+id] = json.RawMessage(doc.Text)
		}
	}
	for i, doubt := range c.Material.Doubts {
		if doubt.Document != nil && doubt.Document.HTML == "" && json.Valid([]byte(doubt.Document.Text)) {
			wanted[fmt.Sprintf("doubt:%d", i)] = json.RawMessage(doubt.Document.Text)
		}
	}
	matches := map[int]struct{ ID, Heading string }{}
	for index, entry := range c.History {
		// A historical page renders findings available through that revision;
		// later review entries retain their original written text.
		commit, _ := entry["entry"].(object)["candidate"].(string)
		if !historyCandidates[commit] {
			continue
		}
		body := entry["entry"].(object)["body"].(string)
		heading, text, ok := strings.Cut(body, ": ")
		if !ok || !strings.HasPrefix(heading, "Doubt ") {
			continue
		}
		if id, ok := byText[strings.Join(strings.Fields(text), " ")]; ok {
			wanted[id] = documents[id]
			matches[index] = struct{ ID, Heading string }{id, heading}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	input, err := json.Marshal(object{"documents": wanted, "sources": c.Sources, "oid": c.Material.ID, "repository": repository})
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "bun", script)
	command.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("render archived review: %w: %s", err, stderr.String())
	}
	var rendered map[string]string
	if err = json.Unmarshal(output, &rendered); err != nil {
		return err
	}
	for id, markup := range rendered {
		if strings.HasPrefix(id, "doubt:") {
			index, err := strconv.Atoi(strings.TrimPrefix(id, "doubt:"))
			if err != nil {
				return err
			}
			c.Material.Doubts[index].Document.HTML = markup
		}
		if strings.HasPrefix(id, "surface:") {
			key := strings.TrimPrefix(id, "surface:")
			doc := c.Documents[key]
			doc.HTML = markup
			c.Documents[key] = doc
		}
	}
	for index, match := range matches {
		c.History[index]["ruling"] = object{"heading": match.Heading, "display": object{"document": object{"rendered_html": rendered[match.ID]}}}
	}
	return nil
}
