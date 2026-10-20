package syncview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"proposal-platform/api"
)

// Store owns serialized keyed views and a bounded window of incremental frames.
// Loading policy: Publish serialized keys only after a committed service transition.
// Caching policy: Keep current keys and at most 256 incremental frames; an expired cursor receives a complete reset.
type Store struct {
	mu      sync.Mutex
	epoch   string
	cursor  uint64
	values  map[string]json.RawMessage
	frames  []api.Frame
	changed chan struct{}
}

func New() *Store {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return &Store{epoch: hex.EncodeToString(id[:]), values: map[string]json.RawMessage{}, changed: make(chan struct{})}
}

func (s *Store) Publish(sequence uint64, puts map[string]json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sequence <= s.cursor {
		panic("non-increasing view sequence")
	}
	owned := make(map[string]json.RawMessage, len(puts))
	for key, value := range puts {
		owned[key] = append(json.RawMessage(nil), value...)
		s.values[key] = owned[key]
	}
	s.cursor = sequence
	s.frames = append(s.frames, api.Frame{Epoch: s.epoch, Cursor: sequence, Puts: owned})
	if len(s.frames) > 256 {
		s.frames = s.frames[1:]
	}
	close(s.changed)
	s.changed = make(chan struct{})
}

func matches(key string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func (s *Store) read(epoch string, after uint64, prefixes []string) api.Frame {
	reset := epoch != s.epoch || after > s.cursor
	if len(s.frames) > 0 && after < s.frames[0].Cursor-1 {
		reset = true
	}
	frame := api.Frame{Epoch: s.epoch, Cursor: s.cursor, Reset: reset, Puts: map[string]json.RawMessage{}}
	if reset {
		for key, value := range s.values {
			if matches(key, prefixes) {
				frame.Puts[key] = append(json.RawMessage(nil), value...)
			}
		}
	} else {
		for _, delta := range s.frames {
			if delta.Cursor > after {
				for key, value := range delta.Puts {
					if matches(key, prefixes) {
						frame.Puts[key] = append(json.RawMessage(nil), value...)
					}
				}
			}
		}
	}
	return frame
}

func (s *Store) Read(epoch string, after uint64, prefixes []string) api.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(epoch, after, prefixes)
}

func (s *Store) Wait(ctx context.Context, epoch string, after uint64, prefixes []string, duration time.Duration) api.Frame {
	if duration < 0 {
		duration = 0
	}
	if duration > 20*time.Second {
		duration = 20 * time.Second
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		s.mu.Lock()
		frame := s.read(epoch, after, prefixes)
		changed := s.changed
		s.mu.Unlock()
		if frame.Reset || frame.Cursor != after {
			return frame
		}
		select {
		case <-changed:
			continue
		case <-ctx.Done():
			return frame
		case <-timer.C:
			return frame
		}
	}
}

// Position reads the event cursor without copying any presentation values.
func (s *Store) Position() api.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return api.Frame{Epoch: s.epoch, Cursor: s.cursor}
}
