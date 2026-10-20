package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-platform/syncview"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/workflow"
)

// Service owns serialized durable workflow state, slot reservations and feed cursor.
// Loading policy: Replay the workflow journal on Open and consume the decision service feed by its durable cursor.
// Caching policy: Keep current proposals, ledger and slot reservations; publish only keys changed by a committed command.
type Service struct {
	policies     map[string]workflow.Policy
	mu           sync.Mutex
	journal      *journal.Journal
	proposals    map[string]*workflow.Proposal
	slots        map[string]string
	ledger       *ledger.Ledger
	feedJournal  string
	feedJournals map[string]bool
	feedCursor   uint64
	head         uint64
	viewCount    uint64
	Views        *syncview.Store
	Wake         chan struct{}
}

func Open(j *journal.Journal, policies map[string]workflow.Policy) (*Service, error) {
	s := &Service{policies: policies, journal: j, proposals: map[string]*workflow.Proposal{}, slots: map[string]string{}, ledger: ledger.New(), Views: syncview.New(), Wake: make(chan struct{}, 1), feedJournals: map[string]bool{}}
	for {
		records, err := j.Read(s.head, 128)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			var e Event
			if err = json.Unmarshal(record.Payload, &e); err != nil {
				return nil, err
			}
			if e.Version != 1 {
				return nil, fmt.Errorf("unsupported workflow event version")
			}
			if e.Ledger != nil {
				if err := e.Ledger.Validate(); err != nil {
					return nil, fmt.Errorf("invalid journal ledger: %w", err)
				}
			}
			s.apply(e, record.Sequence)
		}
	}
	// A process may have performed external work before dying. Never silently rerun it.
	for uid, current := range s.proposals {
		if current.Status == "running" && current.Task != nil && current.Phase != "deliver" {
			p := current.Clone()
			id := "recovery/" + p.Task.ID
			p.Task = nil
			p.Status = "stopped"
			p.Reason = "in-flight operation interrupted; inspect effects and resume explicitly"
			e := Event{Kind: "recovered", Proposals: map[string]*workflow.Proposal{uid: p}, Keys: []string{"proposal/" + uid}}
			if _, err := s.commit(id, id, e); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}

func (s *Service) duplicate(id, digest string) (api.Receipt, bool, error) {
	r, found, err := s.journal.Lookup(id)
	if err != nil || !found {
		return api.Receipt{}, found, err
	}
	if r.Digest != digest {
		return api.Receipt{}, true, fmt.Errorf("command ID reused with different input")
	}
	var e Event
	if err = json.Unmarshal(r.Payload, &e); err != nil {
		return api.Receipt{}, true, err
	}
	return api.Receipt{Sequence: r.Sequence, Keys: e.Keys}, true, nil
}

func (s *Service) commit(id, digest string, e Event) (api.Receipt, error) {
	e.Version = 1
	sort.Strings(e.Keys)
	payload, err := json.Marshal(e)
	if err != nil {
		return api.Receipt{}, err
	}
	proposal := ""
	if len(e.Proposals) == 1 {
		for uid := range e.Proposals {
			proposal = uid
		}
	}
	record, err := s.journal.Append(journal.Record{ID: id, Digest: digest, Proposal: proposal, Payload: payload})
	if err != nil {
		return api.Receipt{}, err
	}
	var persisted Event
	if err = json.Unmarshal(record.Payload, &persisted); err != nil {
		panic(err)
	}
	s.apply(persisted, record.Sequence)
	return api.Receipt{Sequence: record.Sequence, Keys: e.Keys}, nil
}

func (s *Service) Checkpoint() (string, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.feedJournal, s.feedCursor
}
func (s *Service) ViewCount() uint64 { s.mu.Lock(); defer s.mu.Unlock(); return s.viewCount }
