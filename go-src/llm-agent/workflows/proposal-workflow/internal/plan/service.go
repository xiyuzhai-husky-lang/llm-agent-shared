package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"proposal-platform/journal"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
)

// Service owns plan state, explicit starts and the processes for each instance.
// Loading policy: Replay plan states from the journal on Open; role execution starts only through explicit commands.
// Caching policy: Keep current instance states and running process ownership; persist each state change in the journal.
type Service struct {
	Config          *project.Config
	Journal         *journal.Journal
	Directory       string
	CheckAssignment func(string) error
	mu              sync.Mutex
	states          map[string]*State
	running         map[string]bool
	group           sync.WaitGroup
	sequence        uint64
}
type saved struct {
	Instance string `json:"instance"`
	State    *State `json:"state"`
}
type Start struct {
	Instance   string `json:"instance"`
	Assignment string `json:"assignment"`
	Rank       string `json:"rank"`
	StartAt    string `json:"start_at"`
}

func validInstance(name string) bool { return regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(name) }
func clone(state *State) *State {
	raw, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	var copy State
	if err := json.Unmarshal(raw, &copy); err != nil {
		panic(err)
	}
	return &copy
}
func Open(config *project.Config, check func(string) error) (*Service, error) {
	directory := filepath.Join(config.Main, ".local/go-plan-workflow", config.ProjectKey)
	j, err := journal.Open(filepath.Join(directory, "workflow.sqlite"))
	if err != nil {
		return nil, err
	}
	service := &Service{Config: config, Journal: j, Directory: directory, CheckAssignment: check, states: map[string]*State{}, running: map[string]bool{}}
	for {
		records, err := j.Read(service.sequence, 256)
		if err != nil {
			j.Close()
			return nil, err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			var event saved
			if err := material.Decode(string(record.Payload), &event); err != nil {
				j.Close()
				return nil, err
			}
			if !validInstance(event.Instance) {
				j.Close()
				return nil, fmt.Errorf("invalid journal instance")
			}
			if event.State == nil {
				delete(service.states, event.Instance)
			} else {
				if event.State.Repository != config.Repository || event.State.Main != config.Main {
					j.Close()
					return nil, fmt.Errorf("plan journal repository binding differs")
				}
				service.states[event.Instance] = event.State
			}
			service.sequence = record.Sequence
		}
	}
	return service, nil
}
func (s *Service) Close() error { s.group.Wait(); return s.Journal.Close() }
func (s *Service) persistLocked(instance string, state *State) error {
	event := saved{instance, state}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	digest, err := journal.Digest(event)
	if err != nil {
		return err
	}
	record, err := s.Journal.Append(journal.Record{ID: fmt.Sprintf("plan-%d", s.sequence+1), Digest: digest, Proposal: instance, Payload: raw})
	if err != nil {
		return err
	}
	s.sequence = record.Sequence
	if state == nil {
		delete(s.states, instance)
	} else {
		s.states[instance] = clone(state)
	}
	return nil
}
func (s *Service) save(state *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked(state.Instance, state)
}
func (s *Service) Status(instance string) (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[instance]
	if state == nil {
		return nil, fmt.Errorf("no state for instance %s", instance)
	}
	return clone(state), nil
}
func (s *Service) runDirectory(state *State) string {
	return filepath.Join(s.Directory, "runs", state.Instance, state.RunID)
}
func (s *Service) launchLocked(ctx context.Context, state *State) {
	s.running[state.Instance] = true
	s.group.Add(1)
	go func() {
		defer s.group.Done()
		defer func() { s.mu.Lock(); delete(s.running, state.Instance); s.mu.Unlock() }()
		if err := s.run(ctx, state); err != nil {
			if !errors.Is(err, context.Canceled) {
				state.Phase = "failed"
			}
			state.Error = err.Error()
			state.Result = ""
			state.Question = ""
			state.record(err.Error())
			if saveErr := s.save(state); saveErr != nil {
				fmt.Fprintf(os.Stderr, "plan %s: %v; cannot save failure: %v\n", state.Instance, err, saveErr)
			}
		}
	}()
}
func (s *Service) Start(ctx context.Context, request Start) error {
	if !validInstance(request.Instance) {
		return fmt.Errorf("invalid instance")
	}
	if request.StartAt != "planner" && request.StartAt != "reviewer" {
		return fmt.Errorf("start_at must be planner or reviewer")
	}
	data, err := os.ReadFile(request.Assignment)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("empty assignment")
	}
	if err := s.CheckAssignment(request.Assignment); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[request.Instance] {
		return fmt.Errorf("instance already running")
	}
	if existing := s.states[request.Instance]; existing != nil {
		if existing.Phase == "finished" || existing.Phase == "failed" || existing.Phase == "awaiting_answer" {
			return fmt.Errorf("instance is stopped; use answer, verdict or reset")
		}
		if existing.Assignment != string(data) || existing.Rank.Name != request.Rank || existing.StartAt != request.StartAt {
			return fmt.Errorf("resume assignment, rank or start_at differs")
		}
		s.launchLocked(ctx, clone(existing))
		return nil
	}
	var config Configuration
	raw, err := os.ReadFile(filepath.Join(s.Config.Repository, s.Config.Project, "plan-workflow.toml"))
	if err != nil {
		return err
	}
	if err := toml.NewDecoder(strings.NewReader(string(raw))).DisallowUnknownFields().Decode(&config); err != nil {
		return err
	}
	if !filepath.IsLocal(config.CorrespondenceClass) || !filepath.IsLocal(config.ProjectGate) || len(config.Checks) == 0 {
		return fmt.Errorf("invalid plan configuration")
	}
	for _, check := range config.Checks {
		if !filepath.IsLocal(check) {
			return fmt.Errorf("invalid check path")
		}
		if _, err := os.Stat(filepath.Join(s.Config.Repository, check)); err != nil {
			return err
		}
	}
	if info, err := os.Stat(filepath.Join(s.Config.Repository, "llm-agent/correspondence/classes", config.CorrespondenceClass)); err != nil || !info.IsDir() {
		return fmt.Errorf("missing correspondence class")
	}
	if _, err := os.Stat(filepath.Join(s.Config.Repository, s.Config.Project, config.ProjectGate)); err != nil {
		return err
	}
	raw, err = os.ReadFile(filepath.Join(s.Config.Repository, "projects/common/plan-workflow/mission-ranks.toml"))
	if err != nil {
		return err
	}
	var ranks map[string]Rank
	if err := toml.NewDecoder(strings.NewReader(string(raw))).DisallowUnknownFields().Decode(&ranks); err != nil {
		return err
	}
	rank, ok := ranks[request.Rank]
	if !ok || rank.RoleRoundCap <= 0 || rank.TotalRoundCap <= 0 || rank.DeadlineMinutes <= 0 || rank.Model == "" || rank.Effort == "" {
		return fmt.Errorf("invalid plan mission rank %s", request.Rank)
	}
	rank.Name = request.Rank
	phase := "planner"
	if request.StartAt == "reviewer" {
		phase = "gate"
	}
	state := &State{Repository: s.Config.Repository, Main: s.Config.Main, Instance: request.Instance, RunID: fmt.Sprintf("%d", s.sequence+1), Assignment: string(data), StartAt: request.StartAt, Plan: filepath.Join(s.Config.Repository, "llm-agent/correspondence/examples", config.CorrespondenceClass, request.Instance, "plan.chw.tex"), Configuration: config, Rank: rank, Phase: phase, Round: 1, Rounds: []Round{{Number: 1}}, Sessions: map[string]string{}, Events: []Event{}, Verdicts: []Verdict{}, StartedAt: time.Now().Unix()}
	state.record("round 1 begins")
	if err := os.MkdirAll(s.runDirectory(state), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.runDirectory(state), "assignment.md"), data, 0600); err != nil {
		return err
	}
	if err := s.persistLocked(state.Instance, state); err != nil {
		return err
	}
	s.launchLocked(ctx, state)
	return nil
}
func (s *Service) Action(ctx context.Context, instance, action, kind, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[instance] {
		return fmt.Errorf("instance is running")
	}
	current := s.states[instance]
	if current == nil {
		return fmt.Errorf("no state for instance")
	}
	state := clone(current)
	switch action {
	case "reset":
		return s.persistLocked(instance, nil)
	case "answer":
		if state.Phase != "awaiting_answer" || strings.TrimSpace(text) == "" {
			return fmt.Errorf("answer requires a waiting question and nonempty text")
		}
		state.Result = ""
		state.Question = ""
		state.next(false)
		state.current().UserAnswer = text
		state.record("user answer: " + text)
	case "verdict":
		if !state.awaitsVerdict() {
			return fmt.Errorf("instance is not awaiting a verdict")
		}
		if kind != "accept" && kind != "modify" && kind != "reject" {
			return fmt.Errorf("invalid verdict")
		}
		if kind == "modify" && strings.TrimSpace(text) == "" {
			return fmt.Errorf("Modify requires user words")
		}
		state.Verdicts = append(state.Verdicts, Verdict{state.Round, kind, text})
		state.record("user verdict: " + kind)
		if kind == "modify" {
			state.Result = ""
			state.next(false)
			state.current().UserModification = text
		}
	default:
		return fmt.Errorf("unknown plan action")
	}
	if reflect.DeepEqual(current, state) {
		return nil
	}
	if err := s.persistLocked(instance, state); err != nil {
		return err
	}
	if action == "answer" || kind == "modify" {
		s.launchLocked(ctx, state)
	}
	return nil
}
