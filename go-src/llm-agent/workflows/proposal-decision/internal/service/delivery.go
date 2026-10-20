package service

import (
	"context"
	"fmt"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/decision"
	"proposal-platform/api"
	"proposal-platform/journal"
)

func (s *Service) Deliver(ctx context.Context, delivery api.Delivery) (api.Receipt, error) {
	if delivery.ID == "" {
		return api.Receipt{}, fmt.Errorf("delivery ID is required")
	}
	digest, err := journal.Digest(delivery)
	if err != nil {
		return api.Receipt{}, err
	}
	s.deliveryMu.Lock()
	defer s.deliveryMu.Unlock()
	s.mu.Lock()
	if receipt, found, err := s.duplicate("delivery/"+delivery.ID, digest); found || err != nil {
		s.mu.Unlock()
		return receipt, err
	}
	key := revisionKey(delivery.Candidate.Proposal, delivery.Candidate.ID)
	if s.revisions[key] != nil {
		s.mu.Unlock()
		return api.Receipt{}, fmt.Errorf("candidate identity is already published")
	}
	s.mu.Unlock()
	c, err := s.builder.Build(ctx, delivery.Candidate)
	if err != nil {
		return api.Receipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	material := c.Material
	event := Event{Kind: "delivered", Proposal: material.Proposal, Candidate: material.ID, Material: &material, Signatures: map[string]decision.Signature{}, Keys: []string{"proposal/" + material.Proposal, "candidate/" + material.Proposal + "/" + material.ID, key}}
	previous := s.heads[material.Proposal]
	if previous != "" {
		old := s.revisions[revisionKey(material.Proposal, previous)]
		if old.Verdict != nil && (old.Verdict.Kind == "accept" || old.Verdict.Kind == "reject") {
			return api.Receipt{}, fmt.Errorf("revoke the terminal verdict before delivering another candidate")
		}
		event.Keys = append(event.Keys, revisionKey(material.Proposal, previous))
		if old.Verdict != nil && old.Verdict.Kind == "modify" && old.Verdict.KeepSignatures {
			for id, signature := range old.Signatures {
				h, exists := c.Hunks[id]
				if !exists || !h.MustSign {
					continue
				}
				// Equality is checked as well as the digest; classification is not identity.
				prior := old.Candidate.Hunks[id]
				if candidate.HunkID(prior) != candidate.HunkID(h) || prior.Path != h.Path || prior.Header != h.Header || prior.Before != h.Before || prior.After != h.After {
					continue
				}
				signature.CarriedFrom = previous
				event.Signatures[id] = signature
			}
		}
	}
	for _, h := range material.Hunks {
		event.Keys = append(event.Keys, hunkKey(material.Proposal, material.ID, h.ID))
	}
	for _, d := range material.Doubts {
		event.Keys = append(event.Keys, doubtKey(material.Proposal, material.ID, d.ID))
	}
	for _, d := range material.Documents {
		event.Keys = append(event.Keys, "document/"+material.Proposal+"/"+material.ID+"/"+d.ID)
	}
	return s.commit("delivery/"+delivery.ID, digest, event, c)
}
