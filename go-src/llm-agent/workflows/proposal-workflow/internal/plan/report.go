package plan

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"proposal-workflow/internal/material"
)

type Answer struct {
	Item       int    `json:"item"`
	Resolution string `json:"resolution"`
	Evidence   string `json:"evidence,omitempty"`
	Reason     string `json:"reason,omitempty"`
}
type PlannerReport struct {
	Decision      string   `json:"decision"`
	Answers       []Answer `json:"answers,omitempty"`
	Question      string   `json:"question,omitempty"`
	Problems      []string `json:"problems,omitempty"`
	RuleConflicts []string `json:"rule_conflicts"`
	Protest       *string  `json:"protest"`
}
type Violation struct {
	Artifact string `json:"artifact"`
	PlanLine int    `json:"plan_line"`
	RuleFile string `json:"rule_file"`
	RuleLine int    `json:"rule_line"`
	Why      string `json:"why"`
}
type ReviewerReport struct {
	Violations    []Violation `json:"violations"`
	RuleConflicts []string    `json:"rule_conflicts"`
	Protest       *string     `json:"protest"`
}

func tail(text, marker string, value any) error {
	lines := strings.Split(text, "\n")
	position := -1
	for i, line := range lines {
		if line == marker {
			if position != -1 {
				return fmt.Errorf("repeated %s marker", marker)
			}
			position = i
		}
	}
	if position < 0 {
		return fmt.Errorf("missing %s marker", marker)
	}
	rest := strings.TrimSpace(strings.Join(lines[position+1:], "\n"))
	if !strings.HasPrefix(rest, "{") {
		return fmt.Errorf("%s requires one JSON object", marker)
	}
	return material.Decode(rest, value)
}
func channels(conflicts []string, protest *string) error {
	if conflicts == nil {
		return fmt.Errorf("rule_conflicts must be an array")
	}
	for _, conflict := range conflicts {
		if strings.TrimSpace(conflict) == "" {
			return fmt.Errorf("empty rule conflict")
		}
	}
	if protest != nil && strings.TrimSpace(*protest) == "" {
		return fmt.Errorf("empty protest")
	}
	return nil
}
func parsePlanner(text string, expected int, responseOnly bool) (*PlannerReport, error) {
	var report PlannerReport
	if err := tail(text, "Plan result:", &report); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := tail(text, "Plan result:", &fields); err != nil {
		return nil, err
	}
	required := []string{"decision", "rule_conflicts", "protest"}
	switch report.Decision {
	case "present":
		required = append(required, "answers")
	case "incomplete":
		required = append(required, "answers", "question")
	case "chow_problems":
		required = append(required, "problems")
	}
	if len(fields) != len(required) {
		return nil, fmt.Errorf("fields do not match planner decision")
	}
	for _, key := range required {
		raw, ok := fields[key]
		if !ok || (key != "protest" && string(raw) == "null") {
			return nil, fmt.Errorf("required field %s missing or null", key)
		}
	}
	if err := channels(report.RuleConflicts, report.Protest); err != nil {
		return nil, err
	}
	if responseOnly && report.Decision != "present" {
		return nil, fmt.Errorf("response-only turn requires present")
	}
	switch report.Decision {
	case "chow_problems":
		if len(report.Problems) == 0 || report.Question != "" || len(report.Answers) != 0 {
			return nil, fmt.Errorf("chow_problems requires problems only")
		}
		for _, problem := range report.Problems {
			if strings.TrimSpace(problem) == "" {
				return nil, fmt.Errorf("empty Chow problem")
			}
		}
		return &report, nil
	case "incomplete":
		if strings.TrimSpace(report.Question) == "" {
			return nil, fmt.Errorf("incomplete requires a question")
		}
	case "present":
		if report.Question != "" {
			return nil, fmt.Errorf("present cannot carry a question")
		}
	default:
		return nil, fmt.Errorf("invalid planner decision %q", report.Decision)
	}
	if len(report.Problems) != 0 {
		return nil, fmt.Errorf("problems only belong to chow_problems")
	}
	if len(report.Answers) != expected {
		return nil, fmt.Errorf("expected %d ordered answers", expected)
	}
	for i, answer := range report.Answers {
		if answer.Item != i+1 {
			return nil, fmt.Errorf("answers must cover items once in order")
		}
		if answer.Resolution == "fixed" {
			if strings.TrimSpace(answer.Evidence) == "" || answer.Reason != "" {
				return nil, fmt.Errorf("fixed requires evidence only")
			}
		} else if answer.Resolution == "declined" {
			if strings.TrimSpace(answer.Reason) == "" || answer.Evidence != "" {
				return nil, fmt.Errorf("declined requires reason only")
			}
		} else {
			return nil, fmt.Errorf("invalid resolution")
		}
	}
	return &report, nil
}
func parseReviewer(text string) (*ReviewerReport, error) {
	var report ReviewerReport
	if err := tail(text, "Review result:", &report); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := tail(text, "Review result:", &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["protest"]; !ok {
		return nil, fmt.Errorf("required protest field missing")
	}
	if err := channels(report.RuleConflicts, report.Protest); err != nil {
		return nil, err
	}
	if report.Violations == nil {
		return nil, fmt.Errorf("violations must be an array")
	}
	for _, violation := range report.Violations {
		if (violation.Artifact == "plan" && violation.PlanLine <= 0) || (violation.Artifact == "plan_change" && violation.PlanLine != 0) || (violation.Artifact != "plan" && violation.Artifact != "plan_change") {
			return nil, fmt.Errorf("invalid violation artifact or line")
		}
		if strings.TrimSpace(violation.Why) == "" || violation.RuleLine < 0 {
			return nil, fmt.Errorf("invalid violation reason or rule line")
		}
		if violation.RuleFile == "user" {
			if violation.RuleLine != 0 {
				return nil, fmt.Errorf("user rule requires line zero")
			}
		} else if strings.TrimSpace(violation.RuleFile) == "" || (!strings.Contains(violation.RuleFile, "§") && !filepath.IsLocal(violation.RuleFile)) {
			return nil, fmt.Errorf("rule_file must name a repository-relative rule or document section")
		}
	}
	return &report, nil
}
