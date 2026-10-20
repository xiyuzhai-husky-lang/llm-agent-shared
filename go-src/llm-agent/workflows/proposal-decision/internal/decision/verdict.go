package decision

// Verdict freezes the user's permission and the gate at the time it was given.
type Verdict struct {
	Kind           string `json:"kind"`
	Text           string `json:"text"`
	KeepSignatures bool   `json:"keep_signatures"`
	Gate           Gate   `json:"gate"`
}
