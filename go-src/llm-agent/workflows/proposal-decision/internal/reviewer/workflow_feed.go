package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"proposal-platform/api"
	"proposal-platform/httpjson"
	"proposal-platform/syncview"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WorkflowFeed consumes only the workflow's public read-only feed. Its cache is
// separate from the decision journal; workflow metadata cannot become decisions.
type WorkflowFeed struct {
	mu       sync.RWMutex
	values   map[string]json.RawMessage
	failure  string
	changes  *syncview.Store
	sequence uint64
}
type batchRecord struct {
	ID, Title, Approval, Description string
	StartedAt                        string `json:"started_at"`
	Closed                           bool
	Members                          []string
}
type memberRecord struct{ UID, Batch, Title, Scope, Settlement, Source, Branch string }
type ledgerRecord struct {
	Batches map[string]batchRecord
	Members map[string]memberRecord
	Stack   []string
}

func (s *Server) ConnectWorkflow(ctx context.Context, base string) error {
	if base == "" {
		return nil
	}
	endpoint, err := url.Parse(strings.TrimRight(base, "/") + "/api/views")
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" {
		return fmt.Errorf("invalid workflow URL")
	}
	feed := &WorkflowFeed{values: map[string]json.RawMessage{}, changes: syncview.New()}
	client := &http.Client{Timeout: 25 * time.Second}
	epoch := ""
	var cursor uint64
	read := func() error {
		query := url.Values{"prefix": {"ledger", "proposal/"}, "epoch": {epoch}, "after": {strconv.FormatUint(cursor, 10)}, "wait": {"20"}}
		endpoint.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("workflow feed: HTTP %d", response.StatusCode)
		}
		var frame api.Frame
		if err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(&frame); err != nil {
			return err
		}
		feed.mu.Lock()
		defer feed.mu.Unlock()
		changed := frame.Reset || len(frame.Puts) > 0 || feed.failure != ""
		if frame.Reset {
			feed.values = map[string]json.RawMessage{}
		}
		for key, value := range frame.Puts {
			feed.values[key] = value
		}
		epoch, cursor = frame.Epoch, frame.Cursor
		feed.failure = ""
		if changed {
			feed.sequence++
			feed.changes.Publish(feed.sequence, map[string]json.RawMessage{"workflow": json.RawMessage("true")})
		}
		return nil
	}
	if err := read(); err != nil {
		return err
	}
	s.Workflow = feed
	go func() {
		for ctx.Err() == nil {
			if err := read(); err != nil && ctx.Err() == nil {
				feed.mu.Lock()
				feed.failure = err.Error()
				feed.sequence++
				feed.changes.Publish(feed.sequence, map[string]json.RawMessage{"workflow": json.RawMessage("true")})
				feed.mu.Unlock()
				timer := time.NewTimer(3 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}
	}()
	return nil
}
func (f *WorkflowFeed) snapshot() (ledgerRecord, map[string]string, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var ledger ledgerRecord
	_ = json.Unmarshal(f.values["ledger"], &ledger)
	statuses := map[string]string{}
	for key, value := range f.values {
		if strings.HasPrefix(key, "proposal/") {
			var p struct{ Status string }
			if json.Unmarshal(value, &p) == nil {
				statuses[strings.TrimPrefix(key, "proposal/")] = p.Status
			}
		}
	}
	return ledger, statuses, f.failure
}
func (s *Server) workflowEvents(w http.ResponseWriter, r *http.Request) {
	if s.Workflow == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	after, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
	if err != nil && r.URL.Query().Get("after") != "" {
		httpjson.Error(w, err)
		return
	}
	httpjson.Write(w, 200, s.Workflow.changes.Wait(r.Context(), r.URL.Query().Get("epoch"), after, []string{"workflow"}, 20*time.Second))
}
func (s *Server) workflowBatches() object {
	ledger, statuses, failure := s.Workflow.snapshot()
	candidates := map[string]*Candidate{}
	for _, c := range s.latest() {
		candidates[c.Material.Proposal] = c
	}
	stacked, settled := []object{}, []object{}
	ids := append([]string{}, ledger.Stack...)
	closed := []string{}
	for id, b := range ledger.Batches {
		if b.Closed {
			closed = append(closed, id)
		}
	}
	sort.Slice(closed, func(i, j int) bool {
		a, ea := strconv.Atoi(closed[i])
		b, eb := strconv.Atoi(closed[j])
		if ea == nil && eb == nil {
			return a > b
		}
		return closed[i] > closed[j]
	})
	ids = append(ids, closed...)
	for i, id := range ids {
		b := ledger.Batches[id]
		members := []object{}
		var started any = nullable(b.StartedAt)
		for n, uid := range b.Members {
			m := ledger.Members[uid]
			state := statuses[uid]
			if state == "" {
				state = "not started"
			}
			workflow := state
			if m.Settlement != "" {
				state = m.Settlement
			}
			var branch any = nullable(m.Branch)
			if c := candidates[uid]; c != nil {
				branch = nullable(c.Branch)
				started = nullable(c.Date)
			}
			var settlement any
			if m.Settlement != "" {
				settlement = m.Settlement + " by " + m.Source
			}
			members = append(members, object{"uid": uid, "ordinalLabel": fmt.Sprint(n + 1), "title": m.Title, "scope": m.Scope, "branch": branch, "stateLabel": state, "workflowLabel": workflow, "settlementNotice": settlement})
		}
		state, position := "planned", "on the stack"
		if i == 0 {
			position = "top of the stack"
		}
		if b.Closed {
			state, position = "done", "settled"
		}
		row := object{"id": id, "title": b.Title, "description": nullable(b.Description), "stateLabel": state, "positionLabel": position, "startedAt": started, "approvalNotice": nullable(b.Approval), "members": members}
		if b.Closed {
			settled = append(settled, row)
		} else {
			stacked = append(stacked, row)
		}
	}
	var current any
	if len(ledger.Stack) > 0 {
		b := ledger.Batches[ledger.Stack[0]]
		noun := "members"
		if len(b.Members) == 1 {
			noun = "member"
		}
		current = fmt.Sprintf("%s · planned · %d %s", b.Title, len(b.Members), noun)
	}
	return object{"heading": "Batch stack", "currentBatchLabel": current, "closeLabel": "Back to the review", "failure": nullable(failure), "absenceNotice": nil, "stacked": stacked, "settled": settled}
}
