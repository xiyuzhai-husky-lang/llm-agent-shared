package workflow

import "proposal-platform/api"

// Task is one durable role, gate or delivery request with a stable ID.
type Task struct {
	Orchestrator string         `json:"orchestrator,omitempty"`
	Slot         string         `json:"slot"`
	Diagnostic   string         `json:"diagnostic,omitempty"`
	Verdict      *api.Decision  `json:"verdict,omitempty"`
	Work         Work           `json:"work"`
	Policy       Policy         `json:"policy"`
	ID           string         `json:"id"`
	Proposal     string         `json:"proposal"`
	Title        string         `json:"title"`
	Phase        string         `json:"phase"`
	Purpose      string         `json:"purpose"`
	Directory    string         `json:"directory"`
	Assignment   string         `json:"assignment"`
	Candidate    *api.Candidate `json:"candidate,omitempty"`
	Feedback     []api.Doubt    `json:"feedback"`
	UserText     string         `json:"user_text"`
	Revision     int            `json:"revision"`
}
