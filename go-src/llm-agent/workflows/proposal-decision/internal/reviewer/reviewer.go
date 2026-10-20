// Package reviewer projects delivered candidates into the shared reviewer UI protocol.
// It never reads workflow state or invokes the Rust reviewer. Immutable documents and
// source files are prepared at launch and delivery; clicks read the resulting cache.
package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type object = map[string]any
type Subject struct {
	Number int    `json:"proposal_number"`
	Commit string `json:"candidate_commit"`
}
type Request map[string]json.RawMessage

// Loading policy: Prepare pinned documents, source files and display models at launch and delivery.
// Caching policy: Retain immutable models per candidate; clicks combine them with the current decision projection.
type Server struct {
	actionsMu    sync.Mutex
	Workflow     *WorkflowFeed
	extraSources map[string]string
	Service      *service.Service
	Repository   string
	RenderScript string
	mu           sync.RWMutex
	candidates   map[string]*Candidate
	numbers      map[int]string
}
type Candidate struct {
	Page        *ProjectPage
	ProjectDiff *ProjectDiff
	Order       []api.Hunk
	HunkIndex   map[string]int
	Material    api.Candidate
	Subject     Subject
	Documents   map[string]api.Document
	Sources     map[string]string
	Files       map[string]*File
	Paths       []string
	Notebook    object
	Outcome     object
	Label       string
	Folder      string
	Batch       string
	Ordinal     string
	Date        string
	Branch      string
	History     []object
}

func New(ctx context.Context, app *service.Service, repository, renderScript string) (*Server, error) {
	s := &Server{extraSources: map[string]string{}, Service: app, Repository: repository, RenderScript: renderScript, candidates: map[string]*Candidate{}, numbers: map[int]string{}}
	if err := s.Prepare(ctx); err != nil {
		return nil, err
	}
	return s, nil
}
func number(uid string) int {
	i := strings.LastIndexByte(uid, '_')
	n, err := strconv.Atoi(uid[i+1:])
	if err == nil && n >= 0 && uint64(n) <= 0xffffffff {
		return n
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(uid))
	return 1000000000 + int(h.Sum32()%3294967295)
}
func key(uid, id string) string { return uid + "/" + id }
func (s *Server) Prepare(ctx context.Context) error {
	snap := s.Service.ReviewSnapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range snap.Revisions {
		m := r.Material
		k := key(m.Proposal, m.ID)
		if s.candidates[k] != nil {
			continue
		}
		n := number(m.Proposal)
		if previous, ok := s.numbers[n]; ok && previous != m.Proposal {
			return fmt.Errorf("proposal number %d is shared by %s and %s", n, previous, m.Proposal)
		}
		c, err := prepare(ctx, m, s.Repository)
		if err != nil {
			return fmt.Errorf("prepare reviewer %s: %w", k, err)
		}
		if c.Label == "R0" {
			c.Label = fmt.Sprintf("R%d", r.Index)
		}
		historyCandidates := map[string]bool{m.ID: true}
		for _, earlier := range s.candidates {
			if earlier.Material.Proposal == m.Proposal {
				historyCandidates[earlier.Material.ID] = true
			}
		}
		if err := c.renderHistory(ctx, s.RenderScript, s.Repository, historyCandidates); err != nil {
			return err
		}
		s.numbers[n] = m.Proposal
		s.candidates[k] = c
	}
	return nil
}
func (s *Server) selection(subject Subject) (*Candidate, service.ReviewRevision, service.ReviewSnapshot, error) {
	s.mu.RLock()
	uid := s.numbers[subject.Number]
	s.mu.RUnlock()
	snap := s.Service.ReviewSnapshot(uid)
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.candidates[key(uid, subject.Commit)]
	if c == nil {
		return nil, service.ReviewRevision{}, snap, fmt.Errorf("candidate has not been delivered: %d %s", subject.Number, subject.Commit)
	}
	for _, r := range snap.Revisions {
		if r.Material.Proposal == uid && r.Material.ID == subject.Commit {
			return c, r, snap, nil
		}
	}
	return nil, service.ReviewRevision{}, snap, fmt.Errorf("missing decision state")
}
func decodeRequest(req Request) (string, Subject, error) {
	if len(req) != 1 {
		return "", Subject{}, fmt.Errorf("request requires one component")
	}
	for name, raw := range req {
		var sub Subject
		if err := json.Unmarshal(raw, &sub); err != nil {
			return "", sub, err
		}
		return name, sub, nil
	}
	panic("unreachable")
}
func (s *Server) View(req Request) (any, error) {
	name, sub, err := decodeRequest(req)
	if err != nil {
		return nil, err
	}
	if name == "OverviewSidebar" {
		return s.sidebar(), nil
	}
	if name == "BatchStackPage" {
		return s.batches(), nil
	}
	c, r, snap, err := s.selection(sub)
	if err != nil {
		return nil, err
	}
	switch name {
	case "TitleBar":
		return s.title(c, snap), nil
	case "TabStrip":
		return c.tabs(r), nil
	case "MainTab":
		return c.main(), nil
	case "EffectTab":
		return object{"candidate": sub, "effect": c.surface("effect", "Effect")}, nil
	case "TestsTab":
		if c.stoppedAt("tests") {
			return object{"candidate": sub, "surface": object{"kind": "stopped", "surface": c.surface("tests", "Tests")}}, nil
		}
		return object{"candidate": sub, "surface": object{"kind": "notebook", "notebook": c.Notebook}}, nil
	case "DoubtsTab":
		return c.doubts(r), nil
	case "DiffTab":
		return c.diff(r, snap), nil
	case "ReviewPanel":
		return s.review(c, r, snap), nil
	case "OutcomeTab":
		return c.Outcome, nil
	case "WorkflowTab":
		return s.workflow(c), nil
	default:
		return nil, fmt.Errorf("unknown reviewer component %q", name)
	}
}
func href(c *Candidate) string {
	return fmt.Sprintf("?proposal=%d&candidate=%s", c.Subject.Number, c.Material.ID)
}
func short(oid string) string {
	if len(oid) > 7 {
		return oid[:7]
	}
	return oid
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func stop(kind string) any {
	switch kind {
	case "incomplete":
		return "incomplete"
	case "chow_problems":
		return "chow-problems"
	}
	return nil
}
func (s *Server) latest() []*Candidate {
	snap := s.Service.ReviewSnapshot()
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*Candidate{}
	for _, r := range snap.Revisions {
		if r.Latest {
			if c := s.candidates[key(r.Material.Proposal, r.Material.ID)]; c != nil {
				out = append(out, c)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subject.Number < out[j].Subject.Number })
	return out
}
