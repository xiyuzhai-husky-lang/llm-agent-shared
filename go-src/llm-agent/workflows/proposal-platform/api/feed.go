package api

// Feed is an ordered page; Cursor includes examined records without decisions.
type Feed struct {
	Journal   string     `json:"journal"`
	Cursor    uint64     `json:"cursor"`
	Decisions []Decision `json:"decisions"`
}
