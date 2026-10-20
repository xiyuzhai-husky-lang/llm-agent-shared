package service

import (
	"encoding/json"
	"strconv"
	"strings"
)

func (s *Service) apply(e Event, sequence uint64) {
	for uid, p := range e.Proposals {
		s.proposals[uid] = p
	}
	for slot, uid := range e.Slots {
		if uid == "" {
			delete(s.slots, slot)
		} else {
			s.slots[slot] = uid
		}
	}
	if e.Ledger != nil {
		s.ledger = e.Ledger
	}
	if e.Feed != nil {
		s.feedJournals[e.Feed.Journal] = true
		s.feedJournal = e.Feed.Journal
		s.feedCursor = e.Feed.Cursor
	}
	puts := map[string]json.RawMessage{}
	for _, key := range e.Keys {
		var value any
		parts := strings.Split(key, "/")
		switch parts[0] {
		case "ledger":
			value = s.ledger
		case "proposal":
			value = s.proposals[parts[1]]
		case "slot":
			value = map[string]string{"slot": parts[1], "proposal": s.slots[parts[1]]}
		case "feed":
			value = map[string]any{"journal": s.feedJournal, "cursor": s.feedCursor}
		case "event":
			var task any
			if e.Task != nil {
				task = map[string]any{"id": e.Task.ID, "phase": e.Task.Phase, "purpose": e.Task.Purpose, "revision": e.Task.Revision}
			}
			var result any
			if e.Result != nil {
				result = map[string]any{"ok": e.Result.OK, "detail": e.Result.Detail, "findings": e.Result.Findings, "answers": e.Result.Answers}
			}
			value = map[string]any{"sequence": sequence, "kind": e.Kind, "task": task, "result": result}
		case "decision":
			for _, d := range e.Feed.Decisions {
				if d.Proposal == parts[1] && strconv.FormatUint(d.Sequence, 10) == parts[2] {
					value = d
					break
				}
			}
		default:
			panic("unknown declared workflow view key")
		}
		data, err := json.Marshal(value)
		if err != nil {
			panic(err)
		}
		puts[key] = data
		s.viewCount++
	}
	s.head = sequence
	s.Views.Publish(sequence, puts)
	select {
	case s.Wake <- struct{}{}:
	default:
	}
}
