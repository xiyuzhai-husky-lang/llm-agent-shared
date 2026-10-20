package service

import (
	"fmt"
	"sort"
	"time"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/workflow"
)

func (s *Service) Ready() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	for uid, p := range s.proposals {
		if p.Status == "running" && (p.Task == nil || p.Phase == "deliver") {
			ids = append(ids, uid)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s *Service) Active(task workflow.Task) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.proposals[task.Proposal]
	return p != nil && p.Status == "running" && p.Task != nil && p.Task.ID == task.ID
}

func (s *Service) Begin(uid string) (workflow.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.proposals[uid]
	if current == nil || current.Status != "running" {
		return workflow.Task{}, fmt.Errorf("proposal not runnable")
	}
	if current.Task != nil {
		if current.Phase == "deliver" {
			return *current.Clone().Task, nil
		}
		return workflow.Task{}, fmt.Errorf("task already running")
	}
	p := current.Clone()
	p.Generation++
	task := workflow.Task{Orchestrator: p.Orchestrator, Slot: p.Slot, Diagnostic: p.Reason, ID: fmt.Sprintf("%s-r%d-op%d", uid, p.Revision, p.Generation), Proposal: uid, Title: s.ledger.Members[uid].Title, Work: p.Work, Policy: p.Policy, Verdict: p.Verdict, Phase: p.Phase, Purpose: p.Purpose, Directory: p.Directory, Assignment: p.Assignment, Candidate: p.Candidate, Feedback: p.Feedback, UserText: p.UserText, Revision: p.Revision}
	p.Task = &task
	if time.Now().Unix() > p.StartedAt+int64(p.Policy.DeadlineSeconds) {
		p.DeadlineExceeded = true
	}
	event := Event{Kind: "task-started", Proposals: map[string]*workflow.Proposal{uid: p}, Task: &task, Keys: []string{"proposal/" + uid, "event/" + uid + "/" + task.ID + "-start"}}
	digest, err := journal.Digest(task)
	if err != nil {
		return task, err
	}
	_, err = s.commit("begin/"+task.ID, digest, event)
	return *p.Clone().Task, err
}

func (s *Service) Complete(task workflow.Task, result workflow.Result) (api.Receipt, error) {
	digest, err := journal.Digest(result)
	if err != nil {
		return api.Receipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if receipt, found, err := s.duplicate("complete/"+task.ID, digest); found || err != nil {
		return receipt, err
	}
	current := s.proposals[task.Proposal]
	if current == nil || current.Task == nil || current.Task.ID != task.ID || current.Status != "running" {
		return api.Receipt{}, fmt.Errorf("stale completion")
	}
	p, err := workflow.Advance(current, *current.Task, result)
	if err != nil {
		p = current.Clone()
		p.Status = "stuck"
		p.Task = nil
		p.Reason = err.Error()
	}
	event := Event{Kind: "task-completed", Proposals: map[string]*workflow.Proposal{p.UID: p}, Task: &task, Result: &result, Keys: []string{"proposal/" + p.UID, "event/" + p.UID + "/" + task.ID + "-complete"}}
	if task.Phase == "retire-check" && result.OK {
		source := p.UserText
		if source == "" {
			source = "task/" + task.ID
		}
		next, err := s.ledger.Plan(ledger.Action{Kind: "settle", UID: p.UID, Value: p.Purpose, Source: source})
		if err != nil {
			return api.Receipt{}, err
		}
		next.CloseSettledTop()
		event.Ledger = next
		event.Keys = append(event.Keys, "ledger")
	}
	if task.Phase == "land" && result.OK {
		next, err := s.ledger.Plan(ledger.Action{Kind: "settle", UID: p.UID, Value: "landed", Source: "task/" + task.ID})
		if err != nil {
			return api.Receipt{}, err
		}
		next.CloseSettledTop()
		event.Ledger = next
		event.Keys = append(event.Keys, "ledger")
	}
	if (task.Phase == "land-outcome" || task.Phase == "retire-clean") && result.OK {
		event.Slots = map[string]string{p.Slot: ""}
		event.Keys = append(event.Keys, "slot/"+p.Slot)
	}
	return s.commit("complete/"+task.ID, digest, event)
}
