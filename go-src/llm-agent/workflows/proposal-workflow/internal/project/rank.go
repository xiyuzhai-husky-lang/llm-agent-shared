package project

// Rank owns the model and resource policy in mission-ranks.toml.
type Rank struct {
	Model             string `toml:"model"`
	Effort            string `toml:"reasoning_effort"`
	DeadlineMinutes   int    `toml:"deadline_minutes"`
	WorkerRevisionCap int    `toml:"worker_revision_cap"`
}
