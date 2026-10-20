package plan

import (
	"fmt"
	"time"
)

type Rank struct {
	Name            string `json:"rank" toml:"-"`
	DeadlineMinutes int    `json:"deadline_minutes" toml:"deadline_minutes"`
	RoleRoundCap    int    `json:"role_round_cap" toml:"role_round_cap"`
	TotalRoundCap   int    `json:"total_round_cap" toml:"total_round_cap"`
	Model           string `json:"model" toml:"model"`
	Effort          string `json:"reasoning_effort" toml:"reasoning_effort"`
}
type Configuration struct {
	CorrespondenceClass string   `toml:"correspondence_class" json:"correspondence_class"`
	Checks              []string `toml:"checks" json:"checks"`
	ProjectGate         string   `toml:"project_gate" json:"project_gate"`
}
type Event struct {
	Sequence    int    `json:"sequence"`
	Round       int    `json:"round"`
	Phase       string `json:"phase"`
	Description string `json:"description"`
	At          int64  `json:"at"`
}
type Round struct {
	Number           int             `json:"number"`
	Planner          *PlannerReport  `json:"planner"`
	Reviewer         *ReviewerReport `json:"reviewer"`
	Gate             *GateResult     `json:"gate"`
	ArtifactDigest   string          `json:"artifact_digest,omitempty"`
	ResponseOnly     bool            `json:"response_only"`
	UserAnswer       string          `json:"user_answer,omitempty"`
	UserModification string          `json:"user_modification,omitempty"`
}
type GateResult struct {
	Failures []string `json:"failures"`
}
type Verdict struct {
	Round int    `json:"round"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
}
type State struct {
	Repository       string            `json:"repository"`
	Main             string            `json:"main"`
	RunID            string            `json:"run_id"`
	Instance         string            `json:"instance"`
	Assignment       string            `json:"assignment"`
	StartAt          string            `json:"start_at"`
	Plan             string            `json:"plan"`
	Rank             Rank              `json:"rank"`
	Configuration    Configuration     `json:"configuration"`
	Phase            string            `json:"phase"`
	Round            int               `json:"round"`
	Rounds           []Round           `json:"rounds"`
	Result           string            `json:"result,omitempty"`
	Question         string            `json:"question,omitempty"`
	Error            string            `json:"error,omitempty"`
	StartedAt        int64             `json:"started_at"`
	DeadlineRecorded bool              `json:"deadline_recorded"`
	Sessions         map[string]string `json:"sessions"`
	Events           []Event           `json:"events"`
	Verdicts         []Verdict         `json:"verdicts"`
}

func (s *State) current() *Round { return &s.Rounds[len(s.Rounds)-1] }
func (s *State) record(description string) {
	s.Events = append(s.Events, Event{len(s.Events) + 1, s.Round, s.Phase, description, time.Now().Unix()})
}
func (s *State) finish(result, question string) {
	s.Result = result
	s.Question = question
	s.Phase = "finished"
	if result == "incomplete" {
		s.Phase = "awaiting_answer"
	}
	s.record("result: " + result)
}
func (s *State) next(response bool) {
	s.Round++
	s.Rounds = append(s.Rounds, Round{Number: s.Round, ResponseOnly: response})
	s.Phase = "planner"
	s.record(fmt.Sprintf("round %d begins; response-only: %t", s.Round, response))
}
func (s *State) previousViolations() []Violation {
	for i := len(s.Rounds) - 2; i >= 0; i-- {
		if report := s.Rounds[i].Reviewer; report != nil {
			return report.Violations
		}
	}
	return []Violation{}
}
func (s *State) awaitsVerdict() bool {
	return s.Phase == "finished" && (len(s.Verdicts) == 0 || s.Verdicts[len(s.Verdicts)-1].Round != s.Round)
}
