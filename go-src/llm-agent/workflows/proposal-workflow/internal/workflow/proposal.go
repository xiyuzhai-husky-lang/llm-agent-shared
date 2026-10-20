package workflow

import (
	"encoding/json"
	"proposal-platform/api"
)

// Proposal owns one current role loop; history is in the journal, not this value.
// Loading policy: The workflow service restores the current proposal from journal events and applies explicit transitions.
// Caching policy: Keep one current role loop per proposal; the journal retains its previous states.
type Proposal struct {
	Orchestrator     string         `json:"orchestrator,omitempty"`
	Published        string         `json:"published,omitempty"`
	Work             Work           `json:"work"`
	UID              string         `json:"uid"`
	Batch            string         `json:"batch"`
	Slot             string         `json:"slot"`
	Directory        string         `json:"directory"`
	Assignment       string         `json:"assignment"`
	Policy           Policy         `json:"policy"`
	Status           string         `json:"status"`
	Phase            string         `json:"phase"`
	Purpose          string         `json:"purpose"`
	Reason           string         `json:"reason"`
	Generation       uint64         `json:"generation"`
	Revision         int            `json:"revision"`
	WorkerRevisions  int            `json:"worker_revisions"`
	GateFailures     int            `json:"gate_failures"`
	WrapFailures     int            `json:"wrap_failures"`
	Candidate        *api.Candidate `json:"candidate,omitempty"`
	Reviewed         string         `json:"reviewed,omitempty"`
	Feedback         []api.Doubt    `json:"feedback"`
	UserText         string         `json:"user_text"`
	ContinueAt       string         `json:"continue_at"`
	Task             *Task          `json:"task,omitempty"`
	Verdict          *api.Decision  `json:"verdict,omitempty"`
	StartedAt        int64          `json:"started_at"`
	DeadlineExceeded bool           `json:"deadline_exceeded"`
}

func (p *Proposal) Clone() *Proposal {
	data, _ := json.Marshal(p)
	var copy Proposal
	if err := json.Unmarshal(data, &copy); err != nil {
		panic(err)
	}
	return &copy
}
