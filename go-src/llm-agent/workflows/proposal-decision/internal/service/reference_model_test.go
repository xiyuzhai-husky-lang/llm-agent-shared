package service_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"proposal-decision/internal/service"
	"proposal-platform/api"
)

// The oracle uses whole-state copies and set scans, not production transitions,
// incremental counters, candidate builders, gate functions or persistence.
type referenceRevision struct {
	material   api.Candidate
	signatures map[string]string
	carried    map[string]string
	rulings    map[string]string
	opened     bool
	verdict    string
	keep       bool
}

type referenceModel struct {
	revisions map[string]*referenceRevision
	heads     map[string]string
	receipts  map[string]string
}

func newReference() *referenceModel {
	return &referenceModel{revisions: map[string]*referenceRevision{}, heads: map[string]string{}, receipts: map[string]string{}}
}
func refKey(p, r string) string  { return p + "/" + r }
func refDigest(value any) string { data, _ := json.Marshal(value); return string(data) }

func fixture(proposal, id string, variant int) api.Candidate {
	result := api.Candidate{Proposal: proposal, ID: id, Kind: "complete", Title: "Candidate " + id, Base: "base", Documents: []api.Document{{ID: "main", Kind: "main", Title: "Main", Text: "Immutable document"}}}
	for i := 0; i < 3; i++ {
		content := fmt.Sprintf("new-%d", i)
		if i == 1 {
			content += fmt.Sprint(variant % 3)
		}
		h := api.Hunk{Path: fmt.Sprintf("file%d.go", i), Header: "@@ -1 +1 @@", Before: "old", After: content, MustSign: i != 2}
		data, _ := json.Marshal([]string{h.Path, h.Header, h.Before, h.After})
		sum := sha256.Sum256(data)
		h.ID = hex.EncodeToString(sum[:])
		result.Hunks = append(result.Hunks, h)
	}
	result.Doubts = []api.Doubt{{ID: "d0", Source: "reviewer", Text: "First Doubt", Hunk: result.Hunks[0].ID, Worker: "agreed", Reason: "Will fix"}, {ID: "d1", Source: "nitpicker", Text: "Second Doubt"}}
	if variant%7 == 6 {
		result.Kind = "incomplete"
	}
	return result
}

func (m *referenceModel) deliver(d api.Delivery) bool {
	id := "delivery/" + d.ID
	digest := refDigest(d)
	if previous, ok := m.receipts[id]; ok {
		return previous == digest
	}
	key := refKey(d.Candidate.Proposal, d.Candidate.ID)
	if m.revisions[key] != nil {
		return false
	}
	previous := m.revisions[refKey(d.Candidate.Proposal, m.heads[d.Candidate.Proposal])]
	if previous != nil && (previous.verdict == "accept" || previous.verdict == "reject") {
		return false
	}
	next := &referenceRevision{material: d.Candidate, signatures: map[string]string{}, carried: map[string]string{}, rulings: map[string]string{}}
	if previous != nil && previous.verdict == "modify" && previous.keep {
		for _, h := range next.material.Hunks {
			if !h.MustSign {
				continue
			}
			for _, old := range previous.material.Hunks {
				if old.ID == h.ID && old.Path == h.Path && old.Header == h.Header && old.Before == h.Before && old.After == h.After && previous.signatures[old.ID] != "" {
					next.signatures[h.ID] = previous.signatures[old.ID]
					next.carried[h.ID] = previous.material.ID
				}
			}
		}
	}
	m.revisions[key] = next
	m.heads[d.Candidate.Proposal] = d.Candidate.ID
	m.receipts[id] = digest
	return true
}

func (m *referenceModel) gate(r *referenceRevision) bool {
	if r.material.Kind != "complete" || m.heads[r.material.Proposal] != r.material.ID {
		return false
	}
	required := 0
	for _, h := range r.material.Hunks {
		if h.MustSign {
			required++
			if r.signatures[h.ID] == "" {
				return false
			}
		}
	}
	if required > 0 && !r.opened {
		return false
	}
	for _, d := range r.material.Doubts {
		if r.rulings[d.ID] == "" {
			return false
		}
	}
	return true
}

func (m *referenceModel) command(c service.Command) bool {
	key := "command/" + c.ID
	digest := refDigest(c)
	if prior, ok := m.receipts[key]; ok {
		return prior == digest
	}
	r := m.revisions[refKey(c.Proposal, c.Candidate)]
	if r == nil {
		return false
	}
	switch c.Action {
	case "open_diff":
		r.opened = true
	case "sign":
		found := false
		for _, h := range r.material.Hunks {
			if h.ID == c.Target && h.MustSign {
				found = true
			}
		}
		if !found {
			return false
		}
		if r.signatures[c.Target] == "" {
			r.signatures[c.Target] = c.Candidate
		}
	case "sign_under":
		for _, h := range r.material.Hunks {
			if h.MustSign && r.signatures[h.ID] == "" {
				r.signatures[h.ID] = c.Candidate
			}
		}
	case "rule":
		found := false
		for _, d := range r.material.Doubts {
			if d.ID == c.Target {
				found = true
			}
		}
		if !found {
			return false
		}
		if c.Value == "revoked" {
			delete(r.rulings, c.Target)
		} else {
			r.rulings[c.Target] = c.Value
		}
	case "comment":
		if c.Text == "" {
			return false
		}
	case "verdict":
		if (c.Value == "accept" || c.Value == "modify") && m.heads[c.Proposal] != c.Candidate {
			return false
		}
		if c.Value == "revoke" {
			if r.verdict == "" {
				return false
			}
			r.verdict = ""
			r.keep = false
			break
		}
		if r.verdict != "" {
			return false
		}
		if c.Value == "accept" && !m.gate(r) {
			return false
		}
		r.verdict = c.Value
		r.keep = c.KeepSignatures
	default:
		return false
	}
	m.receipts[key] = digest
	return true
}

func decode[T any](data json.RawMessage) T {
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		panic(err)
	}
	return value
}

func (m *referenceModel) check(values map[string]json.RawMessage) error {
	for key, r := range m.revisions {
		summary := decode[map[string]any](values["revision/"+key])
		gate := summary["gate"].(map[string]any)
		if gate["allowed"] != m.gate(r) {
			return fmt.Errorf("gate mismatch %s: %v", key, gate)
		}
		required := 0
		for _, h := range r.material.Hunks {
			if h.MustSign {
				required++
			}
		}
		if summary["signed"] != float64(len(r.signatures)) || summary["ruled"] != float64(len(r.rulings)) || gate["unsigned"] != float64(required-len(r.signatures)) || gate["unruled"] != float64(len(r.material.Doubts)-len(r.rulings)) {
			return fmt.Errorf("incremental count mismatch %s", key)
		}
		if summary["opened"] != r.opened {
			return fmt.Errorf("opened mismatch")
		}
		if r.verdict == "" {
			if summary["verdict"] != nil {
				return fmt.Errorf("unexpected verdict")
			}
		} else {
			actual := summary["verdict"].(map[string]any)
			if actual["kind"] != r.verdict || actual["keep_signatures"] != r.keep {
				return fmt.Errorf("verdict mismatch")
			}
			if r.verdict == "accept" && !actual["gate"].(map[string]any)["allowed"].(bool) {
				return fmt.Errorf("actual Accept has no frozen passing gate")
			}
		}
		for _, h := range r.material.Hunks {
			actual := decode[struct {
				Hunk      api.Hunk `json:"hunk"`
				Signature *struct {
					Signer string `json:"signer"`
					Origin string `json:"signed_at_candidate"`
					Carry  string `json:"carried_from"`
				} `json:"signature"`
			}](values["hunk/"+key+"/"+h.ID])
			if !reflect.DeepEqual(actual.Hunk, h) {
				return fmt.Errorf("immutable hunk changed")
			}
			origin := r.signatures[h.ID]
			if origin == "" {
				if actual.Signature != nil {
					return fmt.Errorf("unexpected signature")
				}
				continue
			}
			if actual.Signature == nil || actual.Signature.Signer != "Xiyu Zhai" || actual.Signature.Origin != origin || actual.Signature.Carry != r.carried[h.ID] {
				return fmt.Errorf("signature mismatch")
			}
			original := m.revisions[refKey(r.material.Proposal, origin)]
			unchanged := false
			for _, old := range original.material.Hunks {
				if old.ID == h.ID && old.Path == h.Path && old.Header == h.Header && old.Before == h.Before && old.After == h.After {
					unchanged = true
				}
			}
			if !unchanged {
				return fmt.Errorf("carried signature content changed")
			}
		}
		for _, d := range r.material.Doubts {
			actual := decode[struct {
				Doubt  api.Doubt `json:"doubt"`
				Ruling string    `json:"ruling"`
			}](values["doubt/"+key+"/"+d.ID])
			if actual.Ruling != r.rulings[d.ID] || !reflect.DeepEqual(actual.Doubt, d) {
				return fmt.Errorf("Doubt mismatch")
			}
		}
	}
	return nil
}
