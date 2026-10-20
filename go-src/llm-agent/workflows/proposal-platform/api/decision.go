package api

import "encoding/json"

// Decision is a committed user effect, with immutable evidence for verdicts.
type Decision struct {
	Sequence       uint64          `json:"sequence"`
	ID             string          `json:"id"`
	Proposal       string          `json:"proposal"`
	Candidate      string          `json:"candidate"`
	Kind           string          `json:"kind"`
	Target         string          `json:"target,omitempty"`
	Value          string          `json:"value,omitempty"`
	Text           string          `json:"text,omitempty"`
	Signer         string          `json:"signer"`
	KeepSignatures bool            `json:"keep_signatures,omitempty"`
	Gate           json.RawMessage `json:"gate,omitempty"`
	Sustained      []Doubt         `json:"sustained,omitempty"`
	At             string          `json:"at"`
}
