package landing

// Record persists completed landing effects so explicit resume never repeats a push or merge.
type Record struct {
	Review       string `json:"review"`
	Candidate    string `json:"candidate"`
	TargetBefore string `json:"target_before"`
	Merge        string `json:"merge"`
	Step         string `json:"step"`
	Outcome      string `json:"outcome"`
}
