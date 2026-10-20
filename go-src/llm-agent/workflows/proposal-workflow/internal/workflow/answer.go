package workflow

// Answer is the Worker's disposition of one role finding, outside the decision log.
type Answer struct {
	Doubt       string `json:"doubt"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason"`
}
