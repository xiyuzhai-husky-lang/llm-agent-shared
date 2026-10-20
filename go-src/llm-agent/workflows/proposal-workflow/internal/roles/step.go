package roles

// Step is the Worker's strict work-step report, before proposal surfaces exist.
type Step struct {
	Kind     string   `json:"kind"`
	Question string   `json:"question,omitempty"`
	Problems []string `json:"problems,omitempty"`
}
