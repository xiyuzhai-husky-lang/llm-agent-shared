package service

// Cost exposes actual operation counts for deterministic complexity checks.
type Cost struct {
	Builds   uint64 `json:"builds"`
	GitCalls uint64 `json:"git_calls"`
	Views    uint64 `json:"views"`
	Visits   uint64 `json:"visits"`
}
