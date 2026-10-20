package workflow

import "proposal-platform/api"

// Result owns the structured output of one role or gate operation.
type Result struct {
	Abort     bool           `json:"abort,omitempty"`
	Work      *Work          `json:"work,omitempty"`
	Documents []api.Document `json:"documents,omitempty"`
	Present   bool           `json:"present,omitempty"`
	OK        bool           `json:"ok"`
	Detail    string         `json:"detail"`
	Candidate *api.Candidate `json:"candidate,omitempty"`
	Findings  []api.Doubt    `json:"findings"`
	Answers   []Answer       `json:"answers"`
	Halt      bool           `json:"halt"`
	Changed   bool           `json:"changed"`
}
