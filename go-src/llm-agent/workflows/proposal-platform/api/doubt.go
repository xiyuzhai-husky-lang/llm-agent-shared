package api

// Doubt is a role finding; Worker disposition is evidence, never a user ruling.
type Doubt struct {
	Anchors  []Anchor  `json:"anchors,omitempty"`
	Document *Document `json:"document,omitempty"`
	Header   string    `json:"header,omitempty"`
	Question string    `json:"question,omitempty"`
	ID       string    `json:"id"`
	Source   string    `json:"source"`
	Text     string    `json:"text"`
	Path     string    `json:"path,omitempty"`
	Hunk     string    `json:"hunk,omitempty"`
	Worker   string    `json:"worker,omitempty"`
	Reason   string    `json:"reason,omitempty"`
}
