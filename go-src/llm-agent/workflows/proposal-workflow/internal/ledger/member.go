package ledger

// Member owns a proposal's membership and explicit terminal settlement.
type Member struct {
	Ordinal     uint64 `json:"ordinal"`
	OriginBatch string `json:"origin_batch,omitempty"`
	Branch      string `json:"branch,omitempty"`
	UID         string `json:"uid"`
	Batch       string `json:"batch"`
	Title       string `json:"title"`
	Scope       string `json:"scope"`
	Settlement  string `json:"settlement"`
	Source      string `json:"source"`
}
