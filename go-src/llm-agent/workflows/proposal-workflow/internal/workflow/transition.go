package workflow

import (
	"fmt"
	"reflect"

	"proposal-platform/api"
)

// Advance applies one completed task to a copy; it has no effects or storage access.
func Advance(current *Proposal, task Task, result Result) (*Proposal, error) {
	if current.Task == nil || current.Task.ID != task.ID || current.Status != "running" {
		return nil, fmt.Errorf("stale task completion")
	}
	p := current.Clone()
	p.Task = nil
	p.Reason = result.Detail
	if result.Work != nil {
		p.Work = *result.Work
	}
	if p.Candidate != nil && len(result.Documents) > 0 {
		p.Candidate.Documents = append(p.Candidate.Documents, result.Documents...)
	}
	if result.Abort {
		p.Status = "aborted"
		return p, nil
	}
	if !result.OK {
		return failed(p, task.Phase), nil
	}
	switch task.Phase {
	case "preflight":
		p.Phase = "worker"
	case "worker":
		if err := worker(p, result); err != nil {
			return nil, err
		}
	case "worker-step":
		if p.Candidate.Kind != "complete" {
			p.Phase = "wrapup"
		} else {
			p.Phase = "project"
		}
	case "project":
		p.Phase = "outcome"
	case "outcome":
		p.GateFailures = 0
		p.Phase = "commissar"
	case "commissar", "reviewer", "nitpicker":
		if err := findings(p, task.Phase, result); err != nil {
			return nil, err
		}
	case "response":
		if result.Changed || (result.Candidate != nil && !reflect.DeepEqual(result.Candidate, p.Candidate)) {
			p.Status = "stuck"
			p.Reason = "response-only turn changed the candidate"
			break
		}
		if err := answers(p, result.Answers); err != nil {
			return nil, err
		}
		p.Phase = p.ContinueAt
		p.Purpose = ""
		p.Feedback = nil
	case "wrapup":
		if result.Candidate == nil {
			return nil, fmt.Errorf("wrap-up requires candidate material")
		}
		if result.Candidate.Proposal != p.UID || result.Candidate.Kind != p.Candidate.Kind || result.Candidate.Base != p.Candidate.Base || !reflect.DeepEqual(result.Candidate.Hunks, p.Candidate.Hunks) {
			return nil, fmt.Errorf("wrap-up changed settled code or candidate kind")
		}
		p.Reviewed = p.Candidate.ID
		material := *result.Candidate
		material.Doubts = append([]api.Doubt(nil), p.Candidate.Doubts...)
		p.Candidate = &material
		p.Phase = "wrap-gate"
	case "wrap-gate":
		p.Phase = "surfaces"
	case "surfaces":
		if result.Candidate != nil {
			p.Candidate = result.Candidate
		}
		p.Phase = "deliver"
	case "retire-check":
		p.Phase = "retire-clean"
	case "retire-clean":
		p.Status = "settled"
		p.Phase = ""
		p.Work.Sessions = map[string]string{}
	case "land":
		p.Phase = "land-outcome"
	case "land-outcome":
		p.Status = "landed"
		p.Phase = ""
	case "deliver":
		p.Published = p.Candidate.ID
		p.Status = "present"
		p.Phase = ""
		p.Purpose = ""
	default:
		return nil, fmt.Errorf("unknown task phase %q", task.Phase)
	}
	return p, nil
}

func failed(p *Proposal, phase string) *Proposal {
	switch phase {
	case "preflight":
		p.Status = "preflight-failed"
	case "worker-step", "project", "outcome":
		p.GateFailures++
		if p.GateFailures <= p.Policy.GateRepairs {
			p.Phase = "worker"
			p.Purpose = "repair"
		} else {
			p.Status = "stuck"
		}
	case "wrapup", "wrap-gate", "surfaces":
		p.WrapFailures++
		if p.WrapFailures <= p.Policy.WrapRepairs {
			p.Phase = "wrapup"
		} else {
			p.Status = "aborted"
		}
	case "land", "land-outcome":
		p.Status = "stuck"
	case "deliver":
		p.Status = "delivery-failed"
	default:
		p.Status = "stuck"
	}
	return p
}

func worker(p *Proposal, r Result) error {
	if r.Candidate == nil || r.Candidate.ID == "" || r.Candidate.Proposal != p.UID {
		return fmt.Errorf("Worker must provide this proposal's candidate")
	}
	if r.Candidate.Kind != "complete" && r.Candidate.Kind != "incomplete" && r.Candidate.Kind != "chow_problems" {
		return fmt.Errorf("invalid Worker candidate kind")
	}
	if p.Candidate != nil && r.Candidate.ID == p.Candidate.ID && p.Purpose != "revision" {
		if !reflect.DeepEqual(r.Candidate, p.Candidate) {
			return fmt.Errorf("same candidate ID has different material")
		}
		if p.Purpose == "feedback" {
			if err := answers(p, r.Answers); err != nil {
				return err
			}
			p.Phase = p.ContinueAt
			p.Purpose = ""
			p.Feedback = nil
			return nil
		}
	}
	if p.Purpose == "feedback" {
		p.WorkerRevisions++
	}
	material := *r.Candidate
	p.Candidate = &material
	p.Phase = "worker-step"
	p.Purpose = ""
	p.Feedback = nil
	return nil
}

func findings(p *Proposal, role string, r Result) error {

	next := map[string]string{"commissar": "reviewer", "reviewer": "nitpicker", "nitpicker": "wrapup"}[role]
	seen := map[string]bool{}
	for _, d := range p.Candidate.Doubts {
		seen[d.ID] = true
	}
	for _, d := range r.Findings {
		if d.ID == "" || d.Text == "" || seen[d.ID] || d.Worker != "" {
			return fmt.Errorf("invalid or duplicate role finding")
		}
		seen[d.ID] = true
		d.Source = role
		p.Candidate.Doubts = append(p.Candidate.Doubts, d)
	}
	if r.Halt {
		p.Phase = "wrapup"
		p.Purpose = "halted"
		if len(r.Findings) > 0 {
			p.Feedback = append([]api.Doubt(nil), r.Findings...)
			p.ContinueAt = "wrapup"
			p.Phase = "response"
			p.Purpose = "response-only"
		}
		return nil
	}
	if len(r.Findings) == 0 {
		p.Phase = next
		return nil
	}
	p.Feedback = append([]api.Doubt(nil), r.Findings...)
	p.ContinueAt = next
	if p.WorkerRevisions < p.Policy.WorkerRevisions && (!p.Work.Present || role == "commissar") {
		p.Phase = "worker"
		p.Purpose = "feedback"
	} else {
		p.Phase = "response"
		p.Purpose = "response-only"
	}
	return nil
}

func answers(p *Proposal, values []Answer) error {
	if len(values) != len(p.Feedback) {
		return fmt.Errorf("Worker must answer every finding")
	}
	replies := map[string]Answer{}
	for _, a := range values {
		if a.Disposition != "agreed" && a.Disposition != "disagreed" {
			return fmt.Errorf("Worker disposition must be agreed or disagreed")
		}
		if _, exists := replies[a.Doubt]; exists {
			return fmt.Errorf("duplicate Worker answer")
		}
		replies[a.Doubt] = a
	}
	for _, d := range p.Feedback {
		if _, ok := replies[d.ID]; !ok {
			return fmt.Errorf("missing Worker answer to %s", d.ID)
		}
	}
	for i, d := range p.Candidate.Doubts {
		if a, exists := replies[d.ID]; exists {
			p.Candidate.Doubts[i].Worker = a.Disposition
			p.Candidate.Doubts[i].Reason = a.Reason
		}
	}
	return nil
}
