package service

import (
	"context"
	"encoding/json"
	"fmt"
	"proposal-platform/api"
	"time"
)

func (s *Service) Feed(ctx context.Context, identity string, after uint64, wait time.Duration) (api.Feed, error) {
	if wait < 0 {
		wait = 0
	}
	if wait > 20*time.Second {
		wait = 20 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		s.mu.Lock()
		if (identity != "" && identity != s.journal.Identity) || (identity == "" && after != 0) || after > s.head {
			s.mu.Unlock()
			return api.Feed{}, fmt.Errorf("decision feed identity or cursor mismatch")
		}
		result := api.Feed{Journal: s.journal.Identity, Cursor: s.head, Decisions: []api.Decision{}}
		records, err := s.journal.ReadFeed(after, 100)
		if err != nil {
			s.mu.Unlock()
			return result, err
		}
		for _, record := range records {
			var d api.Decision
			if err = json.Unmarshal(record.Feed, &d); err != nil {
				s.mu.Unlock()
				return result, err
			}
			d.Sequence = record.Sequence
			result.Decisions = append(result.Decisions, d)
		}
		if len(records) == 100 {
			result.Cursor = records[len(records)-1].Sequence
		}
		changed := s.changed
		s.mu.Unlock()
		if result.Cursor != after || len(result.Decisions) > 0 {
			return result, nil
		}
		select {
		case <-changed:
			continue
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timer.C:
			return result, nil
		}
	}
}
