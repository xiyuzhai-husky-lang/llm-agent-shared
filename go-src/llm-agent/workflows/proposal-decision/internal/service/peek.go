package service

import "proposal-platform/api"

// PeekMaterial returns immutable delivered context. Chat never reads workflow storage.
func (s *Service) PeekMaterial(proposal, id string) (api.Candidate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.revisions[revisionKey(proposal, id)]
	if r == nil {
		return api.Candidate{}, false
	}
	return r.Candidate.Material, true
}
