package plan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func artifactDigest(plan string) (string, error) {
	hash := sha256.New()
	for _, name := range []string{"plan.chw.tex", "plan.kathy.json", "plan-change.json"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(plan), name))
		if err != nil {
			return "", err
		}
		var length [8]byte
		binary.LittleEndian.PutUint64(length[:], uint64(len(data)))
		hash.Write(length[:])
		hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (s *Service) snapshot(state *State) error {
	digest, err := artifactDigest(state.Plan)
	if err != nil {
		return err
	}
	if recorded := state.current().ArtifactDigest; recorded != "" {
		if recorded != digest {
			return fmt.Errorf("artifacts changed since round snapshot")
		}
		return nil
	}
	for _, name := range []string{"plan.chw.tex", "plan.kathy.json", "plan-change.json"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(state.Plan), name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(s.runDirectory(state), fmt.Sprintf("round-%d-%s", state.Round, name)), raw, 0600); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(state.Plan), "plan-change.json"))
	if err != nil {
		return err
	}
	var change struct {
		Kind        string `json:"kind"`
		LibraryFile string `json:"library_file"`
	}
	if err := json.Unmarshal(raw, &change); err != nil {
		return err
	}
	if change.LibraryFile != "" {
		if !filepath.IsLocal(change.LibraryFile) {
			return fmt.Errorf("invalid library path")
		}
		before, err := os.ReadFile(filepath.Join(s.Config.Repository, change.LibraryFile))
		if os.IsNotExist(err) && change.Kind == "add_command" {
			before = []byte{}
		} else if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(s.runDirectory(state), fmt.Sprintf("round-%d-library.txt", state.Round)), before); err != nil {
			return err
		}
	}
	state.current().ArtifactDigest = digest
	return s.save(state)
}
func (s *Service) gate(ctx context.Context, state *State) (bool, error) {
	command := exec.CommandContext(ctx, filepath.Join(s.Config.Repository, s.Config.Project, state.Configuration.ProjectGate), filepath.Dir(state.Plan))
	command.Dir = s.Config.Repository
	output, err := command.Output()
	result := GateResult{Failures: []string{}}
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 {
			return false, fmt.Errorf("project gate failed: %w; %s", err, output)
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(line) != "" {
				result.Failures = append(result.Failures, line)
			}
		}
		if len(result.Failures) == 0 {
			return false, fmt.Errorf("project gate blocked without reasons")
		}
	}
	state.current().Gate = &result
	state.record(fmt.Sprintf("project gate returned %d failures", len(result.Failures)))
	return len(result.Failures) == 0, s.save(state)
}
func (s *Service) gateRepair(state *State) {
	blocked := 0
	for i := len(state.Rounds) - 1; i >= 0; i-- {
		gate := state.Rounds[i].Gate
		if gate == nil || len(gate.Failures) == 0 {
			break
		}
		blocked++
	}
	if blocked > 2 {
		state.finish("stuck", "")
	} else {
		state.next(false)
	}
}
func (s *Service) run(ctx context.Context, state *State) error {
	for state.Phase != "finished" && state.Phase != "awaiting_answer" {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !state.DeadlineRecorded && time.Now().Unix() >= state.StartedAt+int64(state.Rank.DeadlineMinutes)*60 {
			state.DeadlineRecorded = true
			state.record("advisory deadline passed; run continues")
		}
		switch state.Phase {
		case "planner":
			responseOnly := state.current().ResponseOnly
			if responseOnly {
				if err := s.snapshot(state); err != nil {
					return err
				}
			}
			text, err := s.role(ctx, state, "planner")
			if err != nil {
				return err
			}
			if state.Phase == "finished" {
				break
			}
			report, err := parsePlanner(text, len(state.previousViolations()), responseOnly)
			if err != nil {
				return err
			}
			state.current().Planner = report
			if responseOnly {
				digest, err := artifactDigest(state.Plan)
				if err != nil || digest != state.current().ArtifactDigest {
					state.record("planner changed artifacts in a response-only turn")
				}
				state.finish("stuck", "")
			} else if report.Decision == "chow_problems" {
				state.finish("chow_problems", "")
			} else if report.Decision == "incomplete" {
				state.finish("incomplete", report.Question)
			} else {
				state.Phase = "gate"
				state.record("planner presented artifacts")
			}
		case "gate":
			if _, err := os.Stat(state.Plan); err == nil {
				if _, err := os.Stat(filepath.Join(filepath.Dir(state.Plan), "plan.kathy.json")); err == nil {
					if err := s.snapshot(state); err != nil {
						return err
					}
				}
			}
			passed, err := s.gate(ctx, state)
			if err != nil {
				return err
			}
			if passed {
				if err := s.snapshot(state); err != nil {
					return err
				}
				state.Phase = "reviewer"
				state.record("project gate passed")
			} else {
				s.gateRepair(state)
			}
		case "reviewer":
			if err := s.snapshot(state); err != nil {
				return err
			}
			passed, err := s.gate(ctx, state)
			if err != nil {
				return err
			}
			if !passed {
				s.gateRepair(state)
				break
			}
			text, err := s.role(ctx, state, "plan-reviewer")
			if err != nil {
				return err
			}
			if err := s.snapshot(state); err != nil {
				return err
			}
			if state.Phase == "finished" {
				break
			}
			report, err := parseReviewer(text)
			if err != nil {
				return err
			}
			state.current().Reviewer = report
			state.record(fmt.Sprintf("plan-reviewer returned %d violations", len(report.Violations)))
			if len(report.Violations) == 0 {
				state.finish("clean", "")
			} else {
				reviewed := 0
				for _, round := range state.Rounds {
					if round.Reviewer != nil {
						reviewed++
					}
				}
				state.next(reviewed >= state.Rank.RoleRoundCap || reviewed >= state.Rank.TotalRoundCap)
			}
		default:
			return fmt.Errorf("cannot run phase %s", state.Phase)
		}
		if err := s.save(state); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) prompt(state *State, role string) string {
	violations, _ := json.MarshalIndent(state.previousViolations(), "", "  ")
	prompt := fmt.Sprintf("# Assignment\n\n%s\n\n# Round %d\n\nMission rank: %s\nRole review cap: %d\nTotal review cap: %d\nRepository: %s\nInstance directory: %s\nCorrespondence class: %s\nPlan artifact: %s\n\nCheck list:\n- %s\n\nThis workflow is non-interactive. Do not ask the user directly. Do not commit or run state-changing Git commands.\n\nPrevious reviewer violations (answer each in order):\n%s\n", state.Assignment, state.Round, state.Rank.Name, state.Rank.RoleRoundCap, state.Rank.TotalRoundCap, s.Config.Repository, filepath.Dir(state.Plan), state.Configuration.CorrespondenceClass, state.Plan, strings.Join(state.Configuration.Checks, "\n- "), violations)
	if role == "planner" {
		if state.current().ResponseOnly {
			prompt += "The review cap is reached. This is a response-only turn: answer each item without changing artifacts. Return present with answers; the run ends STUCK.\n"
		} else {
			prompt += "Write plan.chw.tex, plan-change.json and convert Chow to plan.kathy.json. Fix conversion refusals while authoring; return chow_problems if required content cannot be expressed.\n"
		}
		prompt += "User answer verbatim:\n" + state.current().UserAnswer + "\nUser Modify verdict verbatim:\n" + state.current().UserModification + "\n"
		for i := len(state.Rounds) - 2; i >= 0; i-- {
			if gate := state.Rounds[i].Gate; gate != nil {
				if len(gate.Failures) > 0 {
					prompt += "Project gate reasons (not reviewer answer items):\n" + strings.Join(gate.Failures, "\n")
				}
				break
			}
		}
	} else {
		answers, _ := json.Marshal(state.current().Planner)
		prompt += "Review both artifacts without changing them. Planner report:\n" + string(answers)
	}
	return prompt
}
func (s *Service) role(ctx context.Context, state *State, role string) (string, error) {
	limit := 3
	if role == "plan-reviewer" {
		limit = 1
	}
	prompt := s.prompt(state, role)
	for correction := 0; correction <= limit; correction++ {
		text, err := s.executeRole(ctx, state, role, correction, prompt)
		if err != nil {
			return "", err
		}
		if role == "planner" {
			_, err = parsePlanner(text, len(state.previousViolations()), state.current().ResponseOnly)
		} else {
			_, err = parseReviewer(text)
		}
		if err == nil {
			return text, nil
		}
		state.record(fmt.Sprintf("%s malformed tail: %v", role, err))
		if correction == limit {
			state.finish("stuck", "")
			return "", s.save(state)
		}
		prompt = fmt.Sprintf("Your output tail is malformed: %v. Return the same report with its strict JSON tail corrected; preserve findings and decisions. Change no files.", err)
	}
	return "", fmt.Errorf("unreachable role correction")
}
