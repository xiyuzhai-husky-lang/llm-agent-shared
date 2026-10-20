package plan

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"proposal-workflow/internal/command"
	"proposal-workflow/internal/material"
)

func (s *Service) Review(ctx context.Context, instance string) (map[string]any, error) {
	state, err := s.Status(instance)
	if err != nil {
		return nil, err
	}
	status := "Running"
	switch state.Phase {
	case "failed":
		status = "Failed"
	case "awaiting_answer":
		status = "Incomplete: the planner asks a question"
	case "finished":
		switch state.Result {
		case "clean":
			status = "Clean: the last review found no violations"
		case "stuck":
			status = "Stuck: the review cap or the project gate ended the run with items open"
		case "chow_problems":
			status = "Chow problems: the planner could not express the plan"
		}
	}
	doubts, notes := []any{}, []any{}
	note := func(round int, kind, text string) {
		notes = append(notes, map[string]string{"heading": fmt.Sprintf("%s · round %d", kind, round), "text": text})
	}
	for index, round := range state.Rounds {
		answers := []Answer{}
		if index+1 < len(state.Rounds) && state.Rounds[index+1].Planner != nil {
			answers = state.Rounds[index+1].Planner.Answers
		}
		if reviewer := round.Reviewer; reviewer != nil {
			for i, violation := range reviewer.Violations {
				location := fmt.Sprintf("plan line %d", violation.PlanLine)
				if violation.Artifact == "plan_change" {
					location = "plan-change.json"
				}
				rule := fmt.Sprintf("%s:%d", violation.RuleFile, violation.RuleLine)
				if violation.RuleFile == "user" {
					rule = "the user's ruling"
				}
				disposition := "unanswered"
				var answer any
				for _, a := range answers {
					if a.Item == i+1 {
						disposition = a.Resolution
						if disposition == "fixed" {
							answer = a.Evidence
						} else {
							answer = a.Reason
						}
						break
					}
				}
				doubts = append(doubts, map[string]any{"heading": fmt.Sprintf("Reviewer · round %d · item %d", round.Number, i+1), "location": location, "rule": rule, "why": violation.Why, "disposition": disposition, "answer": answer})
			}
			for _, conflict := range reviewer.RuleConflicts {
				note(round.Number, "Reviewer rule conflict", conflict)
			}
			if reviewer.Protest != nil {
				note(round.Number, "Reviewer protest", *reviewer.Protest)
			}
		}
		if planner := round.Planner; planner != nil {
			for _, conflict := range planner.RuleConflicts {
				note(round.Number, "Planner rule conflict", conflict)
			}
			if planner.Protest != nil {
				note(round.Number, "Planner protest", *planner.Protest)
			}
			for _, problem := range planner.Problems {
				note(round.Number, "Chow problem", problem)
			}
		}
	}
	change, err := s.change(ctx, state)
	if err != nil {
		return nil, err
	}
	var verdict any
	for _, v := range state.Verdicts {
		if v.Round == state.Round {
			kind := strings.ToUpper(v.Kind[:1]) + v.Kind[1:]
			verdict = map[string]string{"kind": kind, "text": v.Text}
		}
	}
	var question any
	if state.Question != "" {
		question = state.Question
	}
	return map[string]any{"instance": instance, "round": state.Round, "status": status, "awaits_verdict": state.awaitsVerdict(), "verdict": verdict, "question": question, "doubts": doubts, "notes": notes, "change": change}, nil
}
func (s *Service) change(ctx context.Context, state *State) (any, error) {
	snapshotRound := 0
	for _, round := range state.Rounds {
		if round.ArtifactDigest != "" {
			snapshotRound = round.Number
		}
	}
	changePath := filepath.Join(filepath.Dir(state.Plan), "plan-change.json")
	if snapshotRound > 0 {
		changePath = filepath.Join(s.runDirectory(state), fmt.Sprintf("round-%d-plan-change.json", snapshotRound))
	}
	raw, err := os.ReadFile(changePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var change struct {
		Kind        string `json:"kind"`
		LibraryFile string `json:"library_file"`
		Command     string `json:"command"`
		OldCommand  string `json:"old_command"`
		NewCommand  string `json:"new_command"`
		What        string `json:"what_this_is"`
		Why         string `json:"why_this_is_needed"`
		Description string `json:"description"`
	}
	if err := material.Decode(string(raw), &change); err != nil {
		return nil, err
	}
	if change.Kind == "other" {
		return map[string]string{"kind": "other", "description": change.Description}, nil
	}
	if !filepath.IsLocal(change.LibraryFile) {
		return nil, fmt.Errorf("library_file must be repository-relative")
	}
	libraryPath := filepath.Join(s.Config.Repository, change.LibraryFile)
	if snapshotRound > 0 {
		libraryPath = filepath.Join(s.runDirectory(state), fmt.Sprintf("round-%d-library.txt", snapshotRound))
	}
	beforeBytes, err := os.ReadFile(libraryPath)
	if os.IsNotExist(err) && change.Kind == "add_command" {
		beforeBytes = nil
	} else if err != nil {
		return nil, err
	}
	before, after := string(beforeBytes), ""
	switch change.Kind {
	case "add_command":
		separator := "\n\n"
		if before == "" {
			separator = ""
		} else if strings.HasSuffix(before, "\n") {
			separator = "\n"
		}
		after = before + separator + strings.TrimSpace(change.Command) + "\n"
	case "modify_command", "delete_command":
		old, new := strings.TrimSpace(change.Command), ""
		if change.Kind == "modify_command" {
			old, new = strings.TrimSpace(change.OldCommand), strings.TrimSpace(change.NewCommand)
		}
		if old == "" || strings.Count(before, old) != 1 {
			return nil, fmt.Errorf("command to change must occur exactly once in %s", change.LibraryFile)
		}
		after = strings.Replace(before, old, new, 1)
	default:
		return nil, fmt.Errorf("invalid plan change kind %s", change.Kind)
	}
	input, err := json.Marshal(map[string]string{"path": change.LibraryFile, "before": before, "after": after})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(input)
	cache := filepath.Join(s.runDirectory(state), fmt.Sprintf("change-%x.json", digest))
	rendered, err := os.ReadFile(cache)
	if os.IsNotExist(err) {
		output, err := (command.Runner{}).Run(ctx, s.Config.Repository, nil, string(input), "", filepath.Join(s.Config.Repository, "scripts/proposal-tool.sh"), "decision", "--render-file")
		if err != nil {
			return nil, err
		}
		rendered = []byte(output)
		if !json.Valid(rendered) {
			return nil, fmt.Errorf("invalid rendered change document")
		}
		if err := os.WriteFile(cache, rendered, 0600); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return map[string]any{"kind": change.Kind, "library_file": change.LibraryFile, "document": json.RawMessage(rendered), "what_this_is": change.What, "why_this_is_needed": change.Why}, nil
}
func (s *Service) page(ctx context.Context, instance string) (string, error) {
	state, err := s.Status(instance)
	if err != nil {
		return "", err
	}
	round := 0
	for _, r := range state.Rounds {
		if r.ArtifactDigest != "" {
			round = r.Number
		}
	}
	if round == 0 {
		return "", fmt.Errorf("no round has a plan snapshot")
	}
	directory := filepath.Join(s.runDirectory(state), "page", fmt.Sprintf("round-%d", round))
	if _, err := os.Stat(filepath.Join(directory, "index.html")); err == nil {
		return directory, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	command := exec.CommandContext(ctx, "git", "rev-parse", "--short=11", "HEAD")
	command.Dir = s.Config.Repository
	oid, err := command.Output()
	if err != nil {
		return "", err
	}
	build := exec.CommandContext(ctx, "node", filepath.Join(s.Config.Repository, "llm-agent/gadgets/kathy-page/build.mjs"), filepath.Join(s.runDirectory(state), fmt.Sprintf("round-%d-plan.kathy.json", round)), strings.TrimSpace(string(oid)), directory)
	build.Dir = s.Config.Repository
	build.Env = append(os.Environ(), "KATHY_PAGE_TITLE=Plan · "+instance)
	if output, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Kathy page build: %w: %s", err, output)
	}
	return directory, nil
}
