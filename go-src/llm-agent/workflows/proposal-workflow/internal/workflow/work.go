package workflow

// Work owns durable execution facts, separate from the user candidate.
type Work struct {
	RetirementTarget string            `json:"retirement_target,omitempty"`
	MustSign         []string          `json:"must_sign,omitempty"`
	MustSignReasons  map[string]string `json:"must_sign_reasons,omitempty"`
	Parent           string            `json:"parent"`
	Base             string            `json:"base"`
	Folder           string            `json:"folder"`
	Sessions         map[string]string `json:"sessions"`
	Reviewed         string            `json:"reviewed"`
	Outcome          string            `json:"outcome"`
	Question         string            `json:"question"`
	Problems         []string          `json:"problems"`
	Present          bool              `json:"present"`
}
