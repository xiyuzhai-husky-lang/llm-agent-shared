package candidate

import "proposal-platform/api"

// Candidate owns immutable material and indexes created at delivery or replay.
type Candidate struct {
	Material api.Candidate
	Hunks    map[string]api.Hunk
	Doubts   map[string]api.Doubt
	Required int
}

func Restore(material api.Candidate) *Candidate {
	c := &Candidate{Material: material, Hunks: map[string]api.Hunk{}, Doubts: map[string]api.Doubt{}}
	for _, h := range material.Hunks {
		c.Hunks[h.ID] = h
		if h.MustSign {
			c.Required++
		}
	}
	for _, d := range material.Doubts {
		c.Doubts[d.ID] = d
	}
	return c
}
