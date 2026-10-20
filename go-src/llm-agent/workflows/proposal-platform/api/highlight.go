package api

// Highlight is immutable server-rendered syntax for both sides of one hunk.
// Its HTML contains only escaped source and span elements with syntax classes.
type Highlight struct {
	Language string `json:"language"`
	Before   string `json:"before"`
	After    string `json:"after"`
}
