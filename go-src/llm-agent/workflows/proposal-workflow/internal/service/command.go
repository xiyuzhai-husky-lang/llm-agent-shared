package service

import "proposal-workflow/internal/ledger"

// Command owns one explicit workflow or ledger operation and its idempotency key.
type Command struct {
	Orchestrator  string         `json:"orchestrator,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	RequestDigest string         `json:"-"` // Original HTTP command, before server defaults; never accepted from JSON.
	Journal       string         `json:"journal,omitempty"`
	Source        string         `json:"source,omitempty"`
	Folder        string         `json:"folder,omitempty"`
	Base          string         `json:"base,omitempty"`
	ID            string         `json:"id"`
	Action        string         `json:"action"`
	UID           string         `json:"uid,omitempty"`
	Slot          string         `json:"slot,omitempty"`
	Directory     string         `json:"directory,omitempty"`
	Assignment    string         `json:"assignment,omitempty"`
	Rank          string         `json:"rank,omitempty"`
	Ledger        *ledger.Action `json:"ledger,omitempty"`
}
