package api

// Document is a server-prepared surface displayed verbatim by the browser.
type Document struct {
	HTML  string `json:"html,omitempty"`
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
}
