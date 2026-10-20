package service_test

import (
	"proposal-platform/api"
	"proposal-workflow/internal/workflow"
	"testing"
)

// The matrix is a state-model witness: only the configured cap and the Worker's
// present decision govern whether a finding can start another editing turn.
func TestExecutionPolicyModel(t *testing.T) {
	for _, role := range []string{"commissar", "reviewer", "nitpicker"} {
		for _, present := range []bool{false, true} {
			for used := 0; used <= 3; used++ {
				candidate := referenceCandidate("p", "code")
				task := workflow.Task{ID: "review", Phase: role, Proposal: "p"}
				p := &workflow.Proposal{UID: "p", Status: "running", Phase: role, Task: &task, Candidate: &candidate, Policy: workflow.Policy{WorkerRevisions: 2}, WorkerRevisions: used, Work: workflow.Work{Present: present}}
				next, err := workflow.Advance(p, task, workflow.Result{OK: true, Findings: []api.Doubt{{ID: "d", Text: "finding"}}})
				if err != nil {
					t.Fatal(err)
				}
				want := "response"
				if used < 2 && (!present || role == "commissar") {
					want = "worker"
				}
				if next.Phase != want {
					t.Fatalf("role=%s present=%v used=%d: %s != %s", role, present, used, next.Phase, want)
				}
			}
		}
	}
}

func TestHaltAndDeliveryModel(t *testing.T) {
	material := referenceCandidate("p", "code")
	task := workflow.Task{ID: "commissar", Proposal: "p", Phase: "commissar"}
	p := &workflow.Proposal{UID: "p", Status: "running", Phase: task.Phase, Task: &task, Candidate: &material}
	p, err := workflow.Advance(p, task, workflow.Result{OK: true, Halt: true, Findings: []api.Doubt{{ID: "conviction", Text: "halt reason"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != "running" || p.Phase != "response" || len(p.Candidate.Doubts) != 1 {
		t.Fatal("Halt must obtain the Worker's response without another editing turn")
	}
	task = workflow.Task{ID: "halt-response", Proposal: "p", Phase: "response"}
	p.Task = &task
	p, err = workflow.Advance(p, task, workflow.Result{OK: true, Answers: []workflow.Answer{{Doubt: "conviction", Disposition: "agreed", Reason: "acknowledged"}}})
	if err != nil || p.Phase != "wrapup" || p.Candidate.Doubts[0].Worker != "agreed" {
		t.Fatal("Halt lost the response before wrap-up", err)
	}
	task = workflow.Task{ID: "surfaces", Proposal: "p", Phase: "surfaces"}
	p.Task = &task
	p.Phase = "surfaces"
	delivered := *p.Candidate
	delivered.ID = "wrapped"
	delivered.Source = &api.Source{Base: "base", Commit: "wrapped", Repository: "slot"}
	p, err = workflow.Advance(p, task, workflow.Result{OK: true, Candidate: &delivered})
	if err != nil {
		t.Fatal(err)
	}
	if p.Candidate.ID != "wrapped" || p.Phase != "deliver" {
		t.Fatal("the materialized commit must be the delivered candidate")
	}
	p = workflow.Decide(p, api.Decision{Kind: "verdict", Value: "modify", Candidate: "wrapped"})
	if p.Work.Parent != "wrapped" || p.Work.Outcome != "" || p.Phase != "preflight" {
		t.Fatal("Modify must start a fresh measurement based on the published candidate")
	}
}
