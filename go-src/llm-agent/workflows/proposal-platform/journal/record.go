package journal

import "encoding/json"

// Record is one application event; payload and feed are immutable JSON values.
type Record struct {
	Sequence uint64
	ID       string
	Digest   string
	Proposal string
	Payload  json.RawMessage
	Feed     json.RawMessage
}
