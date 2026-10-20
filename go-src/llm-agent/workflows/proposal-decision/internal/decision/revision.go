package decision

import "proposal-decision/internal/candidate"

// Revision owns mutable decisions and incremental counts for one immutable candidate.
// Loading policy: Create decision state for delivered material, then apply committed journal events.
// Caching policy: Maintain signatures, rulings and their counts incrementally for each immutable revision.
type Revision struct {
	Candidate  *candidate.Candidate
	Signatures map[string]Signature
	Rulings    map[string]string
	Verdict    *Verdict
	Opened     bool
	Signed     int
	Ruled      int
}

func New(c *candidate.Candidate) *Revision {
	return &Revision{Candidate: c, Signatures: map[string]Signature{}, Rulings: map[string]string{}}
}

func (r *Revision) Rule(id, value string) {
	previous := r.Rulings[id]
	if value == "revoked" {
		delete(r.Rulings, id)
		if previous != "" {
			r.Ruled--
		}
		return
	}
	if previous == "" {
		r.Ruled++
	}
	r.Rulings[id] = value
}

func (r *Revision) Sign(id string, signature Signature) {
	if _, exists := r.Signatures[id]; exists {
		return
	}
	r.Signatures[id] = signature
	r.Signed++
}
