package service

// Command is an idempotent user input scoped to a proposal candidate.
type Command struct {
	ID             string `json:"id"`
	Proposal       string `json:"proposal"`
	Candidate      string `json:"candidate"`
	Action         string `json:"action"`
	Target         string `json:"target,omitempty"`
	Value          string `json:"value,omitempty"`
	Text           string `json:"text,omitempty"`
	KeepSignatures bool   `json:"keep_signatures,omitempty"`
}
