package service

import (
	"fmt"
	"strconv"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/workflow"
)

func (s *Service) Consume(feed api.Feed) (api.Receipt, error) {
	digest, err := journal.Digest(feed)
	if err != nil {
		return api.Receipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if feed.Journal == "" || (s.feedJournal != "" && s.feedJournal != feed.Journal) {
		return api.Receipt{}, fmt.Errorf("decision journal identity changed")
	}
	if feed.Cursor == s.feedCursor && s.feedJournal != "" && len(feed.Decisions) == 0 {
		return api.Receipt{Sequence: s.head, Keys: []string{}}, nil
	}
	id := "feed/" + feed.Journal + "/" + strconv.FormatUint(feed.Cursor, 10)
	if receipt, found, err := s.duplicate(id, digest); found || err != nil {
		return receipt, err
	}
	if feed.Cursor < s.feedCursor {
		return api.Receipt{}, fmt.Errorf("feed cursor moved backwards")
	}
	if feed.Cursor == s.feedCursor && s.feedJournal != "" {
		return api.Receipt{Sequence: s.head, Keys: []string{}}, nil
	}
	e := Event{Kind: "decisions-consumed", Proposals: map[string]*workflow.Proposal{}, Feed: &feed, Keys: []string{"feed"}}
	declared := map[string]bool{"feed": true}
	previous := s.feedCursor
	for _, d := range feed.Decisions {
		if d.Sequence <= previous || d.Sequence > feed.Cursor {
			return api.Receipt{}, fmt.Errorf("feed must be strictly ordered within its cursor")
		}
		previous = d.Sequence
		p := e.Proposals[d.Proposal]
		if p == nil {
			p = s.proposals[d.Proposal]
		}
		if p == nil {
			continue
		}
		// Settled ledger members never restart because of a delayed feed item.
		if member := s.ledger.Members[d.Proposal]; member.Settlement == "" && d.Kind == "verdict" {
			e.Proposals[d.Proposal] = workflow.Decide(p, d)
			declared["proposal/"+d.Proposal] = true
		}
		declared["decision/"+d.Proposal+"/"+strconv.FormatUint(d.Sequence, 10)] = true
	}
	e.Keys = e.Keys[:0]
	for key := range declared {
		e.Keys = append(e.Keys, key)
	}
	return s.commit(id, digest, e)
}

// A lost decision journal is never silently reset. An operator may connect an
// empty replacement only when every previously delivered proposal is settled.
func (s *Service) connectDecisions(c Command, e *Event) error {
	if !label(c.Journal) || c.Source == "" || s.feedJournals[c.Journal] {
		return fmt.Errorf("connecting a new decision journal requires its identity and source")
	}
	for uid, p := range s.proposals {
		if s.ledger.Members[uid].Settlement != "" {
			continue
		}
		if p.Published != "" || p.Verdict != nil || p.Status == "present" || p.Status == "accepted" || p.Status == "interacting" || p.Status == "rejected" || p.Phase == "deliver" {
			return fmt.Errorf("settle previously published proposal %s before changing decision journals", uid)
		}
	}
	e.Feed = &api.Feed{Journal: c.Journal, Cursor: 0}
	e.Source = c.Source
	e.Keys = []string{"feed"}
	return nil
}
