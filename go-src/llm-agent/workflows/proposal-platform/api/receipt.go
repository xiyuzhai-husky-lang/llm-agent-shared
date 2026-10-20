package api

// Receipt acknowledges one durable command and declares its affected views.
type Receipt struct {
	Sequence uint64   `json:"sequence"`
	Keys     []string `json:"keys"`
	Next     string   `json:"next,omitempty"`
}
