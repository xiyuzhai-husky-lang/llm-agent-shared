package reviewer

import (
	"encoding/json"
	"fmt"
	"maps"
	"proposal-decision/internal/service"
	"regexp"
	"strings"
)

func (s *Server) title(c *Candidate, snap service.ReviewSnapshot) object {
	candidates := []object{}
	for _, r := range snap.Revisions {
		if r.Material.Proposal == c.Material.Proposal && (r.Material.Kind == "complete" || r.Latest) {
			s.mu.RLock()
			other := s.candidates[key(r.Material.Proposal, r.Material.ID)]
			s.mu.RUnlock()
			candidates = append(candidates, object{"label": other.Label, "stop": stop(r.Material.Kind), "candidate": r.Material.ID, "candidate_short": short(r.Material.ID), "href": href(other), "selected": r.Material.ID == c.Material.ID})
		}
	}
	return object{"title": c.Material.Title, "uid": c.Material.Proposal, "candidate": c.Material.ID, "candidateShort": short(c.Material.ID), "candidateLabel": c.Label, "candidateStop": stop(c.Material.Kind), "location": href(c), "candidates": candidates}
}
func (s *Server) sidebar() object {
	groups := [][]object{{}, {}, {}}
	var defaultHref any
	var ledger ledgerRecord
	if s.Workflow != nil {
		ledger, _, _ = s.Workflow.snapshot()
	}
	for _, c := range s.latest() {
		_, r, _, err := s.selection(c.Subject)
		if err != nil {
			continue
		}
		var status any
		group := 0
		if m, ok := ledger.Members[c.Material.Proposal]; ok && m.Settlement != "" {
			status = m.Settlement
			group = 2
		} else if r.Verdict != nil {
			status = r.Verdict.Kind
			group = 2
		}
		if group == 0 && defaultHref == nil {
			defaultHref = href(c)
		}
		groups[group] = append(groups[group], object{"folder": c.Folder, "uid": c.Material.Proposal, "title": c.Material.Title, "ordinalLabel": c.Ordinal + ".", "href": href(c), "dateLabel": c.Date, "branch": nullable(c.Branch), "stop": stop(c.Material.Kind), "candidateKind": c.Material.Kind, "statusLabel": status, "workflowLabel": "present"})
	}
	parts := []object{}
	for i, name := range []string{"Open", "In progress", "Closed"} {
		parts = append(parts, object{"heading": name, "count": len(groups[i]), "foldable": i != 0, "collapsed": i != 0, "rows": groups[i]})
	}
	return object{"title": "Proposals", "batchPageLabel": "Batch stack", "stackNotice": nil, "emptyNotice": nil, "errors": []string{}, "defaultHref": defaultHref, "parts": parts}
}
func (s *Server) batches() object {
	if s.Workflow != nil {
		return s.workflowBatches()
	}
	batches := []object{}
	indices := map[string]int{}
	var current any
	for _, c := range s.latest() {
		id := c.Batch
		if id == "" {
			id = "delivered"
		}
		i, ok := indices[id]
		if !ok {
			i = len(batches)
			indices[id] = i
			batches = append(batches, object{"id": id, "title": c.Material.Title, "description": nil, "stateLabel": "present", "positionLabel": "top of the stack", "startedAt": nullable(c.Date), "approvalNotice": nil, "members": []object{}})
			if current == nil {
				current = id
			}
		}
		b := batches[i]
		b["members"] = append(b["members"].([]object), object{"uid": c.Material.Proposal, "ordinalLabel": c.Ordinal, "title": c.Material.Title, "scope": "Batch scope was not included in the delivery.", "branch": nullable(c.Branch), "stateLabel": "present", "workflowLabel": "present", "settlementNotice": nil})
	}
	return object{"heading": "Batch stack", "currentBatchLabel": current, "closeLabel": "Back to the review", "failure": nil, "absenceNotice": nil, "stacked": batches, "settled": []any{}}
}
func (c *Candidate) tabs(r service.ReviewRevision) object {
	tabs := []object{}
	add := func(id, label, component string, extra ...object) {
		if c.Material.Kind != "complete" && id != "main" {
			if id == "doubts" || id == "diff" || id == "outcome" {
				return
			}
			if _, present := c.Documents[id]; !present && c.Documents["notice"].Title != id {
				return
			}
		}
		h := []object{{"text": label, "tone": "heading"}}
		h = append(h, extra...)
		var marker any
		if notice, ok := c.Documents["notice"]; ok {
			target := notice.Title
			if target != "tests" && target != "effect" {
				target = "main"
			}
			if target == id {
				marker = stop(c.Material.Kind)
			}
		}
		tabs = append(tabs, object{"identifier": id, "heading": h, "component": component, "stop": marker})
	}
	sep := object{"text": " · ", "tone": "separator"}
	text := func(s, t string) object { return object{"text": s, "tone": t} }
	add("main", "Main", "MainTab")
	add("tests", "Tests", "TestsTab", sep, text(fmt.Sprint(c.Notebook["item_count"]), "neutral"))
	add("doubts", "Doubts", "DoubtsTab", sep, text(fmt.Sprintf("%d/%d", len(r.Rulings), len(c.Material.Doubts)), "neutral"))
	added, deleted := 0, 0
	for _, f := range c.Files {
		added += f.Added
		deleted += f.Deleted
	}
	add("diff", "Diff", "DiffTab", sep, text(fmt.Sprint(len(c.Files)), "neutral"), sep, text(fmt.Sprintf("+%d", added), "positive"), text("/", "separator"), text(fmt.Sprintf("−%d", deleted), "negative"), sep, text(fmt.Sprintf("MUST SIGN %d/%d", len(r.Signatures), len(r.Signatures)+r.Gate.Unsigned), "must_sign"))
	add("effect", "Effect", "EffectTab")
	add("outcome", "Outcome", "OutcomeTab")
	var question any
	if notice, ok := c.Documents["notice"]; ok {
		question = "main"
		if notice.Title == "tests" || notice.Title == "effect" {
			question = notice.Title
		}
	}
	return object{"candidate": c.Subject, "default_tab": "main", "question_tab": question, "tabs": tabs}
}
func (c *Candidate) history() []object {
	entries := []object{}
	record := c.Documents["historical-user-review"].Text
	headings := regexp.MustCompile(`(?m)^### (.+)$`).FindAllStringSubmatchIndex(record, -1)
	for i, m := range headings {
		head := record[m[2]:m[3]]
		end := len(record)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		body := strings.TrimSpace(record[m[1]:end])
		fields := strings.Split(head, " · ")
		at := fields[0]
		var candidate, permission any
		keep := false
		role := "reviewer"
		for _, part := range fields[1:] {
			if strings.HasPrefix(part, "Candidate: ") {
				candidate = strings.TrimPrefix(part, "Candidate: ")
			}
			if strings.HasPrefix(part, "Permission: ") {
				permission = strings.ToLower(strings.TrimPrefix(part, "Permission: "))
			}
			if part == "Worker" {
				role = "worker"
			}
			if strings.Contains(part, "Keep signatures") {
				keep = true
			}
		}
		entries = append(entries, object{"entry": object{"timestamp": at, "candidate": candidate, "permission": permission, "keep_signatures": keep, "role": role, "body": body}, "ruling": nil})
	}
	return entries
}
func (s *Server) review(c *Candidate, r service.ReviewRevision, snap service.ReviewSnapshot) object {
	entries := []object{}
	for _, original := range c.History {
		entry := maps.Clone(original)
		entry["entry"] = maps.Clone(original["entry"].(object))
		entries = append(entries, entry)
	}
	for _, entry := range snap.Entries {
		if entry.Proposal != c.Material.Proposal || entry.Kind == "sign" || entry.Kind == "sign_under" {
			continue
		}
		body := entry.Text
		var permission any
		if entry.Kind == "verdict" {
			permission = entry.Value
		}
		var ruling any
		if entry.Kind == "rule" {
			for i, d := range c.Material.Doubts {
				if d.ID == entry.Target {
					ruling = object{"heading": fmt.Sprintf("Doubt %d %s", i+1, entry.Value), "display": object{"document": doubtDocument(d)}}
				}
			}
		}
		entries = append(entries, object{"entry": object{"timestamp": entry.At, "candidate": entry.Candidate, "permission": permission, "keep_signatures": entry.KeepSignatures, "role": "reviewer", "body": body}, "ruling": ruling})
	}
	rounds := []object{}
	var anchor any
	for _, each := range snap.Revisions {
		if each.Material.Proposal != c.Material.Proposal {
			continue
		}
		s.mu.RLock()
		item := s.candidates[key(each.Material.Proposal, each.Material.ID)]
		s.mu.RUnlock()
		actions := []object{}
		for _, permission := range []string{"accept", "modify", "reject", "interact", "revoke"} {
			var block any
			blockAt := func(kind, message string) { block = object{"kind": kind, "message": message} }
			if permission == "revoke" {
				if each.Verdict == nil {
					blockAt("no_standing_verdict", "No standing verdict to revoke")
				}
			} else if (permission == "accept" || permission == "modify") && !each.Latest {
				latest := snap.Revisions[len(snap.Revisions)-1]
				s.mu.RLock()
				label := s.candidates[key(latest.Material.Proposal, latest.Material.ID)].Label
				s.mu.RUnlock()
				blockAt("not_latest", "open "+label+" first")
			} else if each.Verdict != nil {
				blockAt("standing_verdict", "Revoke the standing verdict first")
			} else if permission == "accept" && !each.Gate.Allowed {
				kind := "unsigned_hunks"
				if !each.Gate.Complete {
					kind = "stopped"
				} else if !each.Gate.DiffOpened {
					kind = "signature_prerequisite"
				} else if each.Gate.Unsigned == 0 {
					kind = "undecided_findings"
				}
				message := strings.Join(each.Gate.Reasons, "; ")
				if kind == "unsigned_hunks" {
					message = fmt.Sprintf("%d unsigned", each.Gate.Unsigned)
				}
				blockAt(kind, message)
			}
			actions = append(actions, object{"permission": permission, "label": strings.ToUpper(permission[:1]) + permission[1:], "blocked_because": block})
		}
		var standing any
		keeps := false
		if each.Verdict != nil {
			standing = each.Verdict.Kind
			keeps = each.Verdict.KeepSignatures
		}
		for index, entry := range entries {
			if entry["entry"].(object)["candidate"] == each.Material.ID {
				anchor = index
			}
		}
		if !each.Latest && each.Material.Kind != "complete" {
			actions = []object{}
		}
		rounds = append(rounds, object{"candidate": each.Material.ID, "candidate_short": short(each.Material.ID), "label": item.Label, "stop": stop(each.Material.Kind), "href": href(item), "selected": each.Material.ID == c.Material.ID, "latest": each.Latest, "decided": each.Verdict != nil && (each.Verdict.Kind == "accept" || each.Verdict.Kind == "reject"), "anchor_after_entry": anchor, "standing": standing, "keeps_signatures": keeps, "requires_diff_opening": each.Material.Kind == "complete" && each.Gate.Unsigned+len(each.Signatures) > 0, "keep_signatures_enabled": each.Latest, "actions": actions})
	}
	source, recordPath := "Decision journal", "Decision journal"
	if original, ok := c.Documents["historical-user-review"]; ok {
		source, recordPath = "Runtime record", original.Title
	}
	return object{"candidate": c.Subject, "entries": entries, "rounds": rounds, "source": source, "record_path": recordPath, "signer": snap.Signer}
}

type runRole struct {
	Role    string  `json:"role"`
	Seconds float64 `json:"seconds"`
	Exit    int     `json:"exit"`
	Error   *string `json:"error"`
}

func (s *Server) workflow(c *Candidate) object {
	rounds := []object{}
	for _, d := range c.Material.Documents {
		if !strings.HasPrefix(d.ID, "original-round-") {
			continue
		}
		var raw struct {
			Number    int       `json:"number"`
			Worker    *runRole  `json:"worker"`
			Commissar *runRole  `json:"commissar"`
			Reviewer  *runRole  `json:"reviewer"`
			Nitpicker *runRole  `json:"nitpicker"`
			Attempts  []runRole `json:"attempts"`
			Gates     []struct {
				Kind    string  `json:"kind"`
				Passed  bool    `json:"passed"`
				Detail  string  `json:"detail"`
				Seconds float64 `json:"seconds"`
			} `json:"gates"`
		}
		if json.Unmarshal([]byte(d.Text), &raw) != nil {
			continue
		}
		entries := []object{}
		total := 0.0
		add := func(label string, passed bool, seconds float64, detail string, lines []string) {
			mark := "passed"
			if !passed {
				mark = "failed"
			}
			entries = append(entries, object{"mark": mark, "title": label, "note": nullable(detail), "duration": fmt.Sprintf("%.0fs", seconds), "lines": lines, "children": []any{}})
			total += seconds
		}
		role := func(label, document string, r *runRole) {
			if r == nil {
				return
			}
			detail := ""
			if r.Error != nil {
				detail = *r.Error
			}
			lines := []string{}
			if doc, ok := c.Documents[document]; ok {
				lines = strings.Split(doc.Text, "\n")
			}
			add(label, r.Exit == 0 && r.Error == nil, r.Seconds, detail, lines)
		}
		for i := range raw.Attempts {
			role("Worker", "original-worker", &raw.Attempts[i])
		}
		role("Commissar", "original-commissar", raw.Commissar)
		role("Reviewer", "original-reviewer", raw.Reviewer)
		role("Nitpicker", "original-nitpicker", raw.Nitpicker)
		label := "Worker"
		document := "original-worker"
		if len(raw.Attempts) > 0 {
			label = "Wrap-up"
			document = "original-wrap-up"
		}
		role(label, document, raw.Worker)
		for _, g := range raw.Gates {
			add(g.Kind, g.Passed, g.Seconds, g.Detail, []string{})
		}
		rounds = append(rounds, object{"label": fmt.Sprintf("Round %d", raw.Number), "total": fmt.Sprintf("%.0fs", total), "entries": entries})
	}
	if len(rounds) == 0 {
		rounds = append(rounds, object{"label": "Delivery", "total": "", "entries": []object{{"mark": "event", "title": "Candidate delivered", "note": c.Material.ID, "duration": nil, "lines": []string{}, "children": []any{}}}})
	}
	header := object{"rank": "", "deadline": "", "deadline_passed": false, "elapsed": "", "worker_revisions": object{"usage": c.Label, "reached": false}, "models": []any{}, "delivery": c.Material.ID}
	if s.Workflow != nil {
		s.Workflow.mu.RLock()
		data := s.Workflow.values["proposal/"+c.Material.Proposal]
		s.Workflow.mu.RUnlock()
		var state struct {
			Published        string
			WorkerRevisions  int  `json:"worker_revisions"`
			DeadlineExceeded bool `json:"deadline_exceeded"`
			Policy           struct {
				Rank, Model, Effort string
				WorkerRevisions     int `json:"worker_revisions"`
				DeadlineSeconds     int `json:"deadline_seconds"`
			}
		}
		if json.Unmarshal(data, &state) == nil && state.Published == c.Material.ID {
			header["rank"] = "Rank " + state.Policy.Rank
			header["deadline"] = fmt.Sprintf("Budget %ds", state.Policy.DeadlineSeconds)
			header["deadline_passed"] = state.DeadlineExceeded
			header["worker_revisions"] = object{"usage": fmt.Sprintf("%d/%d", state.WorkerRevisions, state.Policy.WorkerRevisions), "reached": state.WorkerRevisions >= state.Policy.WorkerRevisions}
			header["models"] = []object{{"label": "Worker", "model": state.Policy.Model + " · " + state.Policy.Effort}}
		}
	}
	return object{"header": header, "rounds": rounds}
}
