package material

// Addition names one nontrivial changed file and the Worker's reason.
type Addition struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
