package roles

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"proposal-platform/api"
	"proposal-workflow/internal/workflow"
)

// Report retains the role's words and the protocol values extracted from them.
type Report struct {
	Text      string
	Step      Step
	Findings  []api.Doubt
	Conflicts []string
	Answers   []workflow.Answer
	Present   bool
	Halt      bool
}

func Parse(text, role, taskID string, feedback []api.Doubt) (Report, error) {
	r := Report{Text: text}
	if role == "worker" {
		section, err := Section(text, "Step result:")
		if err != nil {
			return r, err
		}
		decoder := json.NewDecoder(strings.NewReader(section))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&r.Step); err != nil {
			return r, fmt.Errorf("Step result: %w", err)
		}
		switch r.Step.Kind {
		case "complete":
			if r.Step.Question != "" || len(r.Step.Problems) > 0 {
				return r, fmt.Errorf("complete step contains a question or Chow problems")
			}
		case "incomplete":
			if strings.TrimSpace(r.Step.Question) == "" {
				return r, fmt.Errorf("incomplete step needs a question")
			}
		case "chow_problems":
			if len(r.Step.Problems) == 0 {
				return r, fmt.Errorf("Chow problems are empty")
			}
		default:
			return r, fmt.Errorf("unknown Step result kind %q", r.Step.Kind)
		}
		matches := regexp.MustCompile(`(?m)^Loop decision: (review|present)\s*$`).FindAllStringSubmatch(text, -1)
		if len(matches) != 1 {
			return r, fmt.Errorf("expected one Loop decision: review|present")
		}
		r.Present = matches[0][1] == "present"
		if len(feedback) > 0 {
			r.Answers, err = ParseAnswers(text, feedback)
			if err != nil {
				return r, err
			}
		}
		return r, nil
	}
	label := "Doubts:"
	if role == "commissar" {
		label = "Convictions:"
		r.Halt = regexp.MustCompile(`(?m)^Halt: .+`).MatchString(text)
	}
	section, err := Section(text, label)
	if err != nil {
		return r, err
	}
	items, err := Numbered(section)
	if err != nil {
		return r, err
	}
	for i, item := range items {
		r.Findings = append(r.Findings, api.Doubt{ID: fmt.Sprintf("%s-%d", taskID, i+1), Source: role, Text: item})
	}
	if role == "reviewer" {
		for _, label := range []string{"Rule conflicts:", "Must sign:"} {
			if _, err = Section(text, label); err != nil {
				return r, err
			}
		}
	}
	if role == "reviewer" {
		body, err := Section(text, "Rule conflicts:")
		if err != nil {
			return r, err
		}
		r.Conflicts, err = Numbered(body)
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

func Section(text, label string) (string, error) {
	lines := strings.Split(text, "\n")
	start := -1
	end := len(lines)
	headings := regexp.MustCompile(`^[A-Z][A-Za-z -]+:`)
	for i, line := range lines {
		if strings.TrimSpace(line) == label {
			if start >= 0 {
				return "", fmt.Errorf("duplicate %s", label)
			}
			start = i + 1
			continue
		}
		if start >= 0 && i >= start && headings.MatchString(line) && end == len(lines) {
			end = i
		}
	}
	if start < 0 {
		return "", fmt.Errorf("missing %s", label)
	}
	value := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if value == "" {
		return "", fmt.Errorf("empty %s", label)
	}
	return value, nil
}
func Numbered(text string) ([]string, error) {
	if strings.TrimSpace(text) == "none" {
		return nil, nil
	}
	matches := regexp.MustCompile(`(?m)^([0-9]+)\. `).FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 || matches[0][0] != 0 {
		return nil, fmt.Errorf("expected none or sequential numbered items")
	}
	values := []string{}
	for i, m := range matches {
		number, _ := strconv.Atoi(text[m[2]:m[3]])
		if number != i+1 {
			return nil, fmt.Errorf("nonsequential item number %d", number)
		}
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		value := strings.TrimSpace(text[m[1]:end])
		if value == "" {
			return nil, fmt.Errorf("empty item")
		}
		values = append(values, value)
	}
	return values, nil
}
func ParseAnswers(text string, feedback []api.Doubt) ([]workflow.Answer, error) {
	body, err := Section(text, "Dispositions:")
	if err != nil {
		return nil, err
	}
	items, err := Numbered(body)
	if err != nil {
		return nil, err
	}
	if len(items) != len(feedback) {
		return nil, fmt.Errorf("Worker must answer all %d findings", len(feedback))
	}
	answers := []workflow.Answer{}
	for i, item := range items {
		split := strings.SplitN(item, ":", 2)
		word := strings.TrimSpace(split[0])
		reason := item
		disposition := ""
		switch word {
		case "fixed", "agreed":
			disposition = "agreed"
		case "ignored", "rejected", "declined":
			disposition = "disagreed"
		default:
			return nil, fmt.Errorf("invalid disposition %q", word)
		}
		if len(split) == 2 {
			reason = strings.TrimSpace(split[1])
		}
		if reason == "" {
			return nil, fmt.Errorf("empty disposition reason")
		}
		answers = append(answers, workflow.Answer{Doubt: feedback[i].ID, Disposition: disposition, Reason: reason})
	}
	return answers, nil
}
