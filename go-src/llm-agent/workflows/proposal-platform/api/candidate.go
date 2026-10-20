package api

// Candidate is the complete material crossing the workflow/decision boundary.
// Loading policy: Delivered material is decoded and validated at the service boundary.
// Caching policy: Each revision retains its immutable material in the durable journal and the decision projection.
type Candidate struct {
	Folder    string     `json:"folder,omitempty"`
	Reviewed  string     `json:"reviewed,omitempty"`
	Proposal  string     `json:"proposal"`
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Kind      string     `json:"kind"`
	Base      string     `json:"base"`
	Source    *Source    `json:"source,omitempty"`
	Hunks     []Hunk     `json:"hunks"`
	Doubts    []Doubt    `json:"doubts"`
	Documents []Document `json:"documents"`
}
