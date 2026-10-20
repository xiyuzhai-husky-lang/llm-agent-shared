package workflow

// Policy owns explicit bounds for one run; deadlines are advisory, not cancellation.
type Policy struct {
	Model           string `json:"model"`
	Effort          string `json:"effort"`
	Rank            string `json:"rank"`
	WorkerRevisions int    `json:"worker_revisions"`
	GateRepairs     int    `json:"gate_repairs"`
	WrapRepairs     int    `json:"wrap_repairs"`
	DeadlineSeconds int    `json:"deadline_seconds"`
}
