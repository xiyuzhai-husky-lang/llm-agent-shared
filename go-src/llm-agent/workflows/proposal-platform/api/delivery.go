package api

// Delivery is an idempotent request to publish one immutable candidate.
type Delivery struct {
	ID        string    `json:"id"`
	Candidate Candidate `json:"candidate"`
}
