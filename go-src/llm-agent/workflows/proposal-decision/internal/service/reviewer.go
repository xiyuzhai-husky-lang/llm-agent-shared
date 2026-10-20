package service

import (
	"maps"
	"proposal-decision/internal/decision"
	"proposal-platform/api"
	"sort"
)

// ReviewSnapshot is a consistent read of the delivered material and its decisions.
// Material is immutable. Mutable maps and slices are copied under the service lock.
type ReviewSnapshot struct {
	Sequence  uint64
	Signer    string
	Revisions []ReviewRevision
	Entries   []api.Decision
}
type ReviewRevision struct {
	Material   api.Candidate
	Signatures map[string]decision.Signature
	Rulings    map[string]string
	Verdict    *decision.Verdict
	Gate       decision.Gate
	Opened     bool
	Latest     bool
	Index      int
}

func (s *Service) ReviewSnapshot(proposal ...string) ReviewSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := ReviewSnapshot{Sequence: s.head, Signer: s.signer}
	proposals := []string{}
	if len(proposal) > 0 {
		proposals = append(proposals, proposal[0])
	} else {
		for uid := range s.heads {
			proposals = append(proposals, uid)
		}
		sort.Strings(proposals)
	}
	for _, uid := range proposals {
		for index, id := range s.chain[uid] {
			r := s.revisions[revisionKey(uid, id)]
			latest := s.heads[uid] == id
			out.Revisions = append(out.Revisions, ReviewRevision{Material: r.Candidate.Material, Signatures: maps.Clone(r.Signatures), Rulings: maps.Clone(r.Rulings), Verdict: r.Verdict, Gate: decision.Acceptance(r, latest), Opened: r.Opened, Latest: latest, Index: index})
		}
		out.Entries = append(out.Entries, s.reviewEntries[uid]...)
	}
	return out
}
