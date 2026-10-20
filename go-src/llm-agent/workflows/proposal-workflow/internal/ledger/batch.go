package ledger

// Batch owns one ordered proposal group and its approval state.
type Batch struct {
	ApprovedAt   string   `json:"approved_at,omitempty"`
	ApprovalText string   `json:"approval_text,omitempty"`
	Description  string   `json:"description,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Approval     string   `json:"approval"`
	Closed       bool     `json:"closed"`
	Members      []string `json:"members"`
}
