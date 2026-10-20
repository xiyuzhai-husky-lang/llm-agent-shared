package decision

// Gate is the pure acceptance predicate frozen into each verdict.
type Gate struct {
	Latest     bool     `json:"latest"`
	Complete   bool     `json:"complete"`
	DiffOpened bool     `json:"diff_opened"`
	Unsigned   int      `json:"unsigned"`
	Unruled    int      `json:"unruled"`
	Allowed    bool     `json:"allowed"`
	Reasons    []string `json:"reasons"`
}

func Acceptance(r *Revision, latest bool) Gate {
	g := Gate{Latest: latest, Complete: r.Candidate.Material.Kind == "complete", DiffOpened: r.Opened || r.Candidate.Required == 0, Unsigned: r.Candidate.Required - r.Signed, Unruled: len(r.Candidate.Doubts) - r.Ruled, Reasons: []string{}}
	if !g.Latest {
		g.Reasons = append(g.Reasons, "Open the latest candidate")
	}
	if !g.Complete {
		g.Reasons = append(g.Reasons, "Candidate is incomplete or reports Chow problems")
	}
	if !g.DiffOpened {
		g.Reasons = append(g.Reasons, "Open the Diff first")
	}
	if g.Unsigned > 0 {
		g.Reasons = append(g.Reasons, "Required hunks remain unsigned")
	}
	if g.Unruled > 0 {
		g.Reasons = append(g.Reasons, "Doubts remain unruled")
	}
	g.Allowed = len(g.Reasons) == 0
	return g
}
