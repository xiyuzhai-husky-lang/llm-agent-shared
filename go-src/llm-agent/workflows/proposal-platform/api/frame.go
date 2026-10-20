package api

import "encoding/json"

// Frame replaces keyed values, or the entire selected cache when Reset is true.
type Frame struct {
	Epoch  string                     `json:"epoch"`
	Cursor uint64                     `json:"cursor"`
	Reset  bool                       `json:"reset"`
	Puts   map[string]json.RawMessage `json:"puts"`
}
