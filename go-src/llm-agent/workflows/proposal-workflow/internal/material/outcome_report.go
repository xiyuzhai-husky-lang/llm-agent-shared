package material

import "encoding/json"

// OutcomeReport is one entry emitted by the Outcome fragment runner.
type OutcomeReport struct {
	Fragment       string          `json:"fragment"`
	File           string          `json:"file,omitempty"`
	Skipped        json.RawMessage `json:"skipped,omitempty"`
	TriggeredBy    json.RawMessage `json:"triggered_by,omitempty"`
	ElapsedSeconds *int            `json:"elapsed_seconds,omitempty"`
}
