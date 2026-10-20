package api

// Hunk is a server-identified unit of exact content and signature scope.
type Hunk struct {
	Highlight *Highlight `json:"highlight,omitempty"`
	Patch     string     `json:"patch,omitempty"`
	OldStart  int        `json:"old_start,omitempty"`
	NewStart  int        `json:"new_start,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	ID        string     `json:"id"`
	Path      string     `json:"path"`
	Header    string     `json:"header"`
	Before    string     `json:"before"`
	After     string     `json:"after"`
	MustSign  bool       `json:"must_sign"`
}
