package api

// Anchor locates one exact hunk or a file-wide concern in the delivered candidate.
type Anchor struct {
	Path   string `json:"path"`
	Header string `json:"header,omitempty"`
	Hunk   string `json:"hunk,omitempty"`
}
