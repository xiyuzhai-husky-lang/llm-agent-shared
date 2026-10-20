package service

import (
	"proposal-platform/api"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/workflow"
)

// Event owns a durable set of replacements and any atomically consumed feed page.
type Event struct {
	Source    string                        `json:"source,omitempty"`
	Version   int                           `json:"version"`
	Kind      string                        `json:"kind"`
	Proposals map[string]*workflow.Proposal `json:"proposals,omitempty"`
	Slots     map[string]string             `json:"slots,omitempty"`
	Ledger    *ledger.Ledger                `json:"ledger,omitempty"`
	Feed      *api.Feed                     `json:"feed,omitempty"`
	Task      *workflow.Task                `json:"task,omitempty"`
	Result    *workflow.Result              `json:"result,omitempty"`
	Keys      []string                      `json:"keys"`
}
