package service

import (
	"proposal-decision/internal/decision"
	"proposal-platform/api"
)

// Event is a versioned, durable effect; carry and verdict evidence are recorded once.
type Event struct {
	Version    int                           `json:"version"`
	Kind       string                        `json:"kind"`
	Proposal   string                        `json:"proposal"`
	Candidate  string                        `json:"candidate"`
	Material   *api.Candidate                `json:"material,omitempty"`
	Signatures map[string]decision.Signature `json:"signatures,omitempty"`
	Target     string                        `json:"target,omitempty"`
	Value      string                        `json:"value,omitempty"`
	Verdict    *decision.Verdict             `json:"verdict,omitempty"`
	Decision   *api.Decision                 `json:"decision,omitempty"`
	Keys       []string                      `json:"keys"`
	Next       string                        `json:"next,omitempty"`
}
