package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/decision"
	"proposal-platform/api"
	"proposal-platform/journal"
)

func (s *Service) Submit(command Command) (api.Receipt, error) {
	if !candidate.Label(command.ID) {
		return api.Receipt{}, fmt.Errorf("invalid command ID")
	}
	if len(command.Text) > 65536 {
		return api.Receipt{}, fmt.Errorf("comment exceeds 64 KiB")
	}
	digest, err := journal.Digest(command)
	if err != nil {
		return api.Receipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if receipt, found, err := s.duplicate("command/"+command.ID, digest); found || err != nil {
		return receipt, err
	}
	r := s.revisions[revisionKey(command.Proposal, command.Candidate)]
	if r == nil {
		return api.Receipt{}, fmt.Errorf("unknown candidate")
	}
	if command.KeepSignatures && (command.Action != "verdict" || command.Value != "modify") {
		return api.Receipt{}, fmt.Errorf("keep signatures belongs only to Modify")
	}
	event := Event{Kind: command.Action, Proposal: command.Proposal, Candidate: command.Candidate, Target: command.Target, Value: command.Value, Keys: []string{revisionKey(command.Proposal, command.Candidate)}}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	effect := api.Decision{ID: command.ID, Proposal: command.Proposal, Candidate: command.Candidate, Kind: command.Action, Target: command.Target, Value: command.Value, Text: command.Text, Signer: s.signer, KeepSignatures: command.KeepSignatures, At: at}
	changed := true
	switch command.Action {
	case "open_diff":
		changed = !r.Opened
	case "sign", "sign_under":
		event.Signatures = map[string]decision.Signature{}
		if command.Action == "sign" {
			h, exists := r.Candidate.Hunks[command.Target]
			if !exists || !h.MustSign {
				return api.Receipt{}, fmt.Errorf("unknown required hunk")
			}
			if _, signed := r.Signatures[h.ID]; !signed {
				event.Signatures[h.ID] = decision.Signature{Signer: s.signer, SignedAtCandidate: command.Candidate, At: at}
			}
		} else {
			for _, h := range r.Candidate.Material.Hunks {
				s.visits++
				if h.MustSign && (command.Target == "" || h.Path == command.Target || strings.HasPrefix(h.Path, strings.TrimSuffix(command.Target, "/")+"/")) {
					if _, signed := r.Signatures[h.ID]; !signed {
						event.Signatures[h.ID] = decision.Signature{Signer: s.signer, SignedAtCandidate: command.Candidate, At: at}
					}
				}
			}
		}
		changed = len(event.Signatures) > 0
		for id := range event.Signatures {
			event.Keys = append(event.Keys, hunkKey(command.Proposal, command.Candidate, id))
		}
		event.Next = s.next(r, command.Target, event.Signatures)
	case "rule":
		if _, exists := r.Candidate.Doubts[command.Target]; !exists {
			return api.Receipt{}, fmt.Errorf("unknown Doubt")
		}
		switch command.Value {
		case "sustained", "dismissed", "followup", "defer", "revoked":
		default:
			return api.Receipt{}, fmt.Errorf("invalid Doubt ruling")
		}
		previous := r.Rulings[command.Target]
		changed = previous != command.Value && !(previous == "" && command.Value == "revoked")
		event.Keys = append(event.Keys, doubtKey(command.Proposal, command.Candidate, command.Target))
	case "comment":
		if strings.TrimSpace(command.Text) == "" {
			return api.Receipt{}, fmt.Errorf("empty comment")
		}
		if command.Target != "" {
			if _, ok := r.Candidate.Hunks[command.Target]; !ok {
				return api.Receipt{}, fmt.Errorf("unknown comment hunk")
			}
		}
		event.Keys = []string{}
	case "verdict":
		if err = s.verdict(r, command, &event, &effect); err != nil {
			return api.Receipt{}, err
		}
	default:
		return api.Receipt{}, fmt.Errorf("unknown action %q", command.Action)
	}
	if !changed {
		event.Kind = "noop"
		event.Keys = []string{}
	}
	if changed && command.Action != "open_diff" {
		event.Decision = &effect
		event.Keys = append(event.Keys, "entry/"+command.Proposal+"/"+command.ID)
	}
	return s.commit("command/"+command.ID, digest, event, nil)
}

func (s *Service) verdict(r *decision.Revision, c Command, e *Event, effect *api.Decision) error {
	switch c.Value {
	case "accept", "modify", "reject", "interact", "revoke":
	default:
		return fmt.Errorf("unknown verdict")
	}
	latest := s.heads[c.Proposal] == c.Candidate
	if (c.Value == "accept" || c.Value == "modify") && !latest {
		return fmt.Errorf("open the latest candidate")
	}
	if c.Value == "revoke" {
		if r.Verdict == nil {
			return fmt.Errorf("no standing verdict")
		}
	} else if r.Verdict != nil {
		return fmt.Errorf("revoke the standing verdict first")
	}
	gate := decision.Acceptance(r, latest)
	if c.Value == "accept" && !gate.Allowed {
		return fmt.Errorf("accept blocked: %s", strings.Join(gate.Reasons, "; "))
	}
	e.Verdict = &decision.Verdict{Kind: c.Value, Text: c.Text, KeepSignatures: c.KeepSignatures, Gate: gate}
	effect.Gate, _ = json.Marshal(gate)
	if c.Value == "modify" {
		for _, doubt := range r.Candidate.Material.Doubts {
			s.visits++
			if r.Rulings[doubt.ID] == "sustained" {
				effect.Sustained = append(effect.Sustained, doubt)
			}
		}
	}
	return nil
}

// next excludes the signed location even if its Doubts are unruled.
func (s *Service) next(r *decision.Revision, target string, added map[string]decision.Signature) string {
	for _, h := range r.Candidate.Material.Hunks {
		s.visits++
		_, signed := r.Signatures[h.ID]
		_, newly := added[h.ID]
		if h.ID != target && h.MustSign && !signed && !newly {
			return hunkKey(r.Candidate.Material.Proposal, r.Candidate.Material.ID, h.ID)
		}
	}
	for _, d := range r.Candidate.Material.Doubts {
		s.visits++
		if d.Hunk != target && r.Rulings[d.ID] == "" {
			return doubtKey(r.Candidate.Material.Proposal, r.Candidate.Material.ID, d.ID)
		}
	}
	return revisionKey(r.Candidate.Material.Proposal, r.Candidate.Material.ID)
}
