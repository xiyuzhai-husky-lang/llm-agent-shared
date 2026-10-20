package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/decision"
	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-platform/syncview"
)

// Service owns durable decision application and keyed in-memory projections.
// Loading policy: Replay the decision journal on Open; build new candidate material only on explicit delivery.
// Caching policy: Keep indexed revisions and keyed views in memory; commands update only affected decision keys.
type Service struct {
	reviewEntries map[string][]api.Decision
	signer        string
	mu            sync.Mutex
	deliveryMu    sync.Mutex
	journal       *journal.Journal
	builder       *candidate.Builder
	revisions     map[string]*decision.Revision
	heads         map[string]string
	chain         map[string][]string
	Views         *syncview.Store
	head          uint64
	changed       chan struct{}
	views         uint64
	visits        uint64
}

func Open(j *journal.Journal, b *candidate.Builder, signer string) (*Service, error) {
	if strings.TrimSpace(signer) == "" {
		return nil, fmt.Errorf("signer must not be empty")
	}
	s := &Service{reviewEntries: map[string][]api.Decision{}, signer: signer, journal: j, builder: b, revisions: map[string]*decision.Revision{}, heads: map[string]string{}, chain: map[string][]string{}, Views: syncview.New(), changed: make(chan struct{})}
	for {
		records, err := j.Read(s.head, 128)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			var event Event
			if err = json.Unmarshal(record.Payload, &event); err != nil {
				return nil, err
			}
			if event.Version != 1 {
				return nil, fmt.Errorf("unsupported decision event version")
			}
			s.apply(event, record.Sequence, nil)
		}
	}
	return s, nil
}

func revisionKey(proposal, id string) string     { return "revision/" + proposal + "/" + id }
func hunkKey(proposal, id, hunk string) string   { return "hunk/" + proposal + "/" + id + "/" + hunk }
func doubtKey(proposal, id, doubt string) string { return "doubt/" + proposal + "/" + id + "/" + doubt }

func (s *Service) duplicate(id, digest string) (api.Receipt, bool, error) {
	record, found, err := s.journal.Lookup(id)
	if err != nil || !found {
		return api.Receipt{}, found, err
	}
	if record.Digest != digest {
		return api.Receipt{}, true, fmt.Errorf("command ID reused with different input")
	}
	var event Event
	if err = json.Unmarshal(record.Payload, &event); err != nil {
		return api.Receipt{}, true, err
	}
	return api.Receipt{Sequence: record.Sequence, Keys: event.Keys, Next: event.Next}, true, nil
}

func (s *Service) commit(id, digest string, event Event, built *candidate.Candidate) (api.Receipt, error) {
	event.Version = 1
	payload, err := json.Marshal(event)
	if err != nil {
		return api.Receipt{}, err
	}
	var feed []byte
	if event.Decision != nil {
		feed, err = json.Marshal(event.Decision)
		if err != nil {
			return api.Receipt{}, err
		}
	}
	record, err := s.journal.Append(journal.Record{ID: id, Digest: digest, Proposal: event.Proposal, Payload: payload, Feed: feed})
	if err != nil {
		return api.Receipt{}, err
	}
	var persisted Event
	if err = json.Unmarshal(record.Payload, &persisted); err != nil {
		panic(err)
	}
	s.apply(persisted, record.Sequence, built)
	return api.Receipt{Sequence: record.Sequence, Keys: event.Keys, Next: event.Next}, nil
}

func (s *Service) Cost() Cost {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Cost{Builds: s.builder.Builds.Load(), GitCalls: s.builder.GitCalls.Load(), Views: s.views, Visits: s.visits}
}
