package service

import (
	"encoding/json"
	"strings"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/decision"
)

func (s *Service) apply(e Event, sequence uint64, built *candidate.Candidate) {
	if e.Decision != nil {
		entry := *e.Decision
		entry.Sequence = sequence
		s.reviewEntries[e.Proposal] = append(s.reviewEntries[e.Proposal], entry)
	}
	key := revisionKey(e.Proposal, e.Candidate)
	r := s.revisions[key]
	switch e.Kind {
	case "delivered":
		if built == nil {
			built = candidate.Restore(*e.Material)
		}
		r = decision.New(built)
		s.revisions[key] = r
		s.heads[e.Proposal] = e.Candidate
		s.chain[e.Proposal] = append(s.chain[e.Proposal], e.Candidate)
		for id, signature := range e.Signatures {
			r.Sign(id, signature)
		}
	case "sign", "sign_under":
		for id, signature := range e.Signatures {
			r.Sign(id, signature)
		}
	case "open_diff":
		r.Opened = true
	case "rule":
		r.Rule(e.Target, e.Value)
	case "verdict":
		if e.Value == "revoke" {
			r.Verdict = nil
		} else {
			r.Verdict = e.Verdict
		}
	}
	puts := map[string]json.RawMessage{}
	for _, key := range e.Keys {
		value := s.project(key, e, sequence)
		encoded, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		puts[key] = encoded
		s.views++
	}
	s.Views.Publish(sequence, puts)
	s.head = sequence
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *Service) project(key string, e Event, sequence uint64) any {
	parts := strings.Split(key, "/")
	if parts[0] == "proposal" {
		return map[string]any{"proposal": e.Proposal, "latest": s.heads[e.Proposal], "candidates": s.chain[e.Proposal]}
	}
	if parts[0] == "entry" {
		copy := *e.Decision
		copy.Sequence = sequence
		return copy
	}
	r := s.revisions[revisionKey(parts[1], parts[2])]
	material := r.Candidate.Material
	switch parts[0] {
	case "revision":
		return map[string]any{"proposal": material.Proposal, "candidate": material.ID, "signed": r.Signed, "required": r.Candidate.Required, "ruled": r.Ruled, "doubts": len(r.Candidate.Doubts), "opened": r.Opened, "verdict": r.Verdict, "gate": decision.Acceptance(r, s.heads[material.Proposal] == material.ID), "signer": s.signer}
	case "candidate":
		hunks := []string{}
		labels := map[string]string{}
		for _, h := range material.Hunks {
			hunks = append(hunks, h.ID)
			labels[h.ID] = h.Path + " " + h.Header
			if h.MustSign {
				labels[h.ID] = "MUST SIGN · " + labels[h.ID]
			}
		}
		doubts := []string{}
		for _, d := range material.Doubts {
			doubts = append(doubts, d.ID)
		}
		docs := []string{}
		for _, d := range material.Documents {
			docs = append(docs, d.ID)
		}
		return map[string]any{"proposal": material.Proposal, "id": material.ID, "title": material.Title, "kind": material.Kind, "base": material.Base, "reviewed": material.Reviewed, "hunks": hunks, "hunk_labels": labels, "doubts": doubts, "documents": docs}
	case "hunk":
		var signature any
		if signed, exists := r.Signatures[parts[3]]; exists {
			signature = signed
		}
		return map[string]any{"hunk": r.Candidate.Hunks[parts[3]], "signature": signature}
	case "doubt":
		return map[string]any{"doubt": r.Candidate.Doubts[parts[3]], "ruling": r.Rulings[parts[3]]}
	case "document":
		for _, d := range material.Documents {
			if d.ID == parts[3] {
				return d
			}
		}
	}
	panic("unknown declared view key: " + key)
}
