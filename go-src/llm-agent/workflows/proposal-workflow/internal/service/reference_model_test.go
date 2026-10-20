package service_test

import (
	"encoding/json"
	"fmt"
	"reflect"

	"proposal-platform/api"
	"proposal-workflow/internal/workflow"
)

// referenceProposal is an executable specification using whole values and scans.
// It does not call production transitions, gates, ledgers, runners or persistence.
type referenceProposal struct {
	uid          string
	status       string
	phase        string
	purpose      string
	generation   uint64
	revision     int
	revisions    int
	cap          int
	failures     int
	wrapFailures int
	candidate    *api.Candidate
	feedback     []api.Doubt
	continueAt   string
	task         *workflow.Task
	verdict      string
}

type referenceWorkflow struct {
	proposals map[string]*referenceProposal
	cursor    uint64
	completed map[string]string
}

func referenceCandidate(uid, id string) api.Candidate {
	return api.Candidate{Proposal: uid, ID: id, Kind: "complete", Base: "base", Hunks: []api.Hunk{{Path: "code.go", Header: "@@", Before: "old", After: "new", MustSign: true}}, Doubts: []api.Doubt{}, Documents: []api.Document{}}
}

func (m *referenceWorkflow) begin(uid string) workflow.Task {
	p := m.proposals[uid]
	p.generation++
	task := workflow.Task{ID: fmt.Sprintf("%s-r%d-op%d", uid, p.revision, p.generation), Proposal: uid, Phase: p.phase, Purpose: p.purpose, Revision: p.revision}
	p.task = &task
	return task
}

func (m *referenceWorkflow) complete(task workflow.Task, r workflow.Result) {
	p := m.proposals[task.Proposal]
	p.task = nil
	data, _ := json.Marshal(r)
	m.completed[task.ID] = string(data)
	if !r.OK {
		if task.Phase == "preflight" {
			p.status = "preflight-failed"
			return
		}
		if task.Phase == "worker-step" || task.Phase == "project" || task.Phase == "outcome" {
			p.failures++
			if p.failures > 2 {
				p.status = "stuck"
			} else {
				p.phase = "worker"
				p.purpose = "repair"
			}
			return
		}
		if task.Phase == "wrapup" || task.Phase == "wrap-gate" || task.Phase == "surfaces" {
			p.wrapFailures++
			if p.wrapFailures > 2 {
				p.status = "aborted"
			} else {
				p.phase = "wrapup"
			}
			return
		}
		if task.Phase == "deliver" {
			p.status = "delivery-failed"
		} else {
			p.status = "stuck"
		}
		return
	}
	switch task.Phase {
	case "preflight":
		p.phase = "worker"
	case "worker":
		if p.candidate != nil && p.candidate.ID == r.Candidate.ID && p.purpose == "feedback" {
			m.answer(p, r.Answers)
			p.phase = p.continueAt
			p.purpose = ""
			p.feedback = nil
		} else {
			if p.purpose == "feedback" {
				p.revisions++
			}
			p.candidate = cloneCandidate(r.Candidate)
			p.phase = "worker-step"
			p.purpose = ""
			p.feedback = nil
		}
	case "worker-step":
		if p.candidate.Kind == "complete" {
			p.phase = "project"
		} else {
			p.phase = "wrapup"
		}
	case "project":
		p.phase = "outcome"
	case "outcome":
		p.phase = "commissar"
		p.failures = 0
	case "commissar", "reviewer", "nitpicker":

		next := "wrapup"
		if task.Phase == "commissar" {
			next = "reviewer"
		}
		if task.Phase == "reviewer" {
			next = "nitpicker"
		}
		for _, finding := range r.Findings {
			finding.Source = task.Phase
			p.candidate.Doubts = append(p.candidate.Doubts, finding)
		}
		if r.Halt {
			p.phase = "wrapup"
			p.purpose = "halted"
			return
		}
		if len(r.Findings) == 0 {
			p.phase = next
			return
		}
		p.feedback = append([]api.Doubt(nil), r.Findings...)
		p.continueAt = next
		if p.revisions < p.cap {
			p.phase = "worker"
			p.purpose = "feedback"
		} else {
			p.phase = "response"
			p.purpose = "response-only"
		}
	case "response":
		if r.Changed || (r.Candidate != nil && !reflect.DeepEqual(r.Candidate, p.candidate)) {
			p.status = "stuck"
			return
		}
		m.answer(p, r.Answers)
		p.phase = p.continueAt
		p.purpose = ""
		p.feedback = nil
	case "wrapup":
		old := p.candidate
		p.candidate = cloneCandidate(r.Candidate)
		p.candidate.Doubts = append([]api.Doubt(nil), old.Doubts...)
		p.phase = "wrap-gate"
	case "wrap-gate":
		p.phase = "surfaces"
	case "surfaces":
		p.phase = "deliver"
	case "deliver":
		p.status = "present"
		p.phase = ""
		p.purpose = ""
	}
}

func (m *referenceWorkflow) answer(p *referenceProposal, answers []workflow.Answer) {
	for i, d := range p.candidate.Doubts {
		for _, answer := range answers {
			if answer.Doubt == d.ID {
				p.candidate.Doubts[i].Worker = answer.Disposition
				p.candidate.Doubts[i].Reason = answer.Reason
			}
		}
	}
}

func cloneCandidate(c *api.Candidate) *api.Candidate {
	if c == nil {
		return nil
	}
	encoded, _ := json.Marshal(c)
	var result api.Candidate
	_ = json.Unmarshal(encoded, &result)
	return &result
}

func (m *referenceWorkflow) decide(d api.Decision) {
	p := m.proposals[d.Proposal]
	if d.Kind != "verdict" || p.candidate == nil || d.Candidate != p.candidate.ID {
		return
	}
	if d.Value == "revoke" {
		if p.verdict == "modify" && p.status == "running" {
			p.status = "stopped"
		} else if p.verdict != "" {
			p.status = "present"
			p.phase = ""
		}
		p.verdict = ""
	} else {
		p.verdict = d.Value
		switch d.Value {
		case "accept":
			p.status = "accepted"
		case "reject":
			p.status = "rejected"
		case "interact":
			p.status = "interacting"
		case "modify":
			p.status = "running"
			p.phase = "preflight"
			p.purpose = "revision"
			p.revision++
			p.revisions = 0
			p.failures = 0
			p.wrapFailures = 0
			p.feedback = d.Sustained
		}
	}
	p.task = nil
	p.generation++
}

func (m *referenceWorkflow) check(values map[string]json.RawMessage) error {
	for uid, p := range m.proposals {
		var actual workflow.Proposal
		if err := json.Unmarshal(values["proposal/"+uid], &actual); err != nil {
			return err
		}
		if actual.Status != p.status || actual.Phase != p.phase || actual.Purpose != p.purpose || actual.Generation != p.generation || actual.Revision != p.revision || actual.WorkerRevisions != p.revisions || actual.GateFailures != p.failures || actual.WrapFailures != p.wrapFailures {
			return fmt.Errorf("%s: actual status=%s phase=%s purpose=%s generation=%d revision=%d worker=%d repairs=%d/%d; model=%+v", uid, actual.Status, actual.Phase, actual.Purpose, actual.Generation, actual.Revision, actual.WorkerRevisions, actual.GateFailures, actual.WrapFailures, p)
		}
		if !reflect.DeepEqual(actual.Candidate, p.candidate) {
			return fmt.Errorf("candidate mismatch for %s: actual=%+v model=%+v", uid, actual.Candidate, p.candidate)
		}
		if (actual.Task == nil) != (p.task == nil) {
			return fmt.Errorf("task ownership mismatch")
		}
		if actual.Task != nil && actual.Task.ID != p.task.ID {
			return fmt.Errorf("task identity mismatch")
		}
	}
	return nil
}
