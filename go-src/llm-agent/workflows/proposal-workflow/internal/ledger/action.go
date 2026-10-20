package ledger

// Action owns one explicit ledger operation.
type Action struct {
	OriginBatch  string `json:"origin_batch,omitempty"`
	Branch       string `json:"branch,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	ApprovedAt   string `json:"approved_at,omitempty"`
	ApprovalText string `json:"approval_text,omitempty"`
	Kind         string `json:"kind"`
	Batch        string `json:"batch,omitempty"`
	UID          string `json:"uid,omitempty"`
	Title        string `json:"title,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Value        string `json:"value,omitempty"`
	Source       string `json:"source,omitempty"`
}
