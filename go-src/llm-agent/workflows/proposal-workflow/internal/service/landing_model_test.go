package service_test

import (
	"encoding/json"
	"testing"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

// A pushed landing stays settled across measurement failure and restart. The
// exclusive slot is released only when the later measurement has completed.
func TestLandingSettlementRecoveryModel(t *testing.T) {
	app, j, path := setup(t)
	defer func() { _ = j.Close() }()
	read := func() workflow.Proposal {
		var p workflow.Proposal
		if err := json.Unmarshal(app.Views.Read("", 0, []string{"proposal/p0"}).Puts["proposal/p0"], &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for i := 0; i < 20 && read().Status != "present"; i++ {
		task, err := app.Begin("p0")
		if err != nil {
			t.Fatal(err)
		}
		result := workflow.Result{OK: true}
		if task.Phase == "worker" {
			value := referenceCandidate("p0", "code")
			result.Candidate = &value
		}
		if task.Phase == "wrapup" {
			value := *task.Candidate
			value.ID = "wrapped"
			result.Candidate = &value
		}
		if _, err = app.Complete(task, result); err != nil {
			t.Fatal(err)
		}
	}
	if read().Status != "present" {
		t.Fatal("fixture failed to present")
	}
	if _, err := app.Submit(service.Command{ID: "unsafe-connect", Action: "connect_decisions", Journal: "new", Source: "test"}); err == nil {
		t.Fatal("journal replacement allowed while a delivered candidate is unsettled")
	}
	if _, err := app.Consume(api.Feed{Journal: "decision", Cursor: 1, Decisions: []api.Decision{{Sequence: 1, Kind: "verdict", Value: "accept", Proposal: "p0", Candidate: "wrapped", Signer: "Test signer"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Submit(service.Command{ID: "premature-settlement", Action: "ledger", Ledger: &ledger.Action{Kind: "settle", UID: "p0", Value: "landed"}}); err == nil {
		t.Fatal("Accept alone must not claim a landed Git change")
	}
	if _, err := app.Submit(service.Command{ID: "land", Action: "land", UID: "p0"}); err != nil {
		t.Fatal(err)
	}
	task, err := app.Begin("p0")
	if err != nil {
		t.Fatal(err)
	}
	if task.Phase != "land" || task.Verdict == nil || task.Verdict.Candidate != "wrapped" {
		t.Fatal("landing lacks frozen acceptance")
	}
	if _, err = app.Complete(task, workflow.Result{OK: false, Detail: "merge stopped"}); err != nil {
		t.Fatal(err)
	}
	if read().Phase != "land" || read().Status != "stuck" {
		t.Fatal("failed merge advanced to measurement")
	}
	if _, err = app.Submit(service.Command{ID: "resume-merge", Action: "resume", UID: "p0"}); err != nil {
		t.Fatal(err)
	}
	task, err = app.Begin("p0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Complete(task, workflow.Result{OK: true, Detail: "merged and pushed"}); err != nil {
		t.Fatal(err)
	}
	if read().Phase != "land-outcome" {
		t.Fatal("landing must schedule a distinct measurement")
	}
	frame := app.Views.Read("", 0, []string{"ledger", "slot/p0"})
	var batch ledger.Ledger
	if err = json.Unmarshal(frame.Puts["ledger"], &batch); err != nil {
		t.Fatal(err)
	}
	if batch.Members["p0"].Settlement != "landed" || batch.Members["p0"].Source != "task/"+task.ID {
		t.Fatal("the pushed landing must be settled before measurement")
	}
	task, err = app.Begin("p0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Complete(task, workflow.Result{OK: false, Detail: "measurement failed"}); err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err = service.Open(j, testPolicies())
	if err != nil {
		t.Fatal(err)
	}
	if read().Phase != "land-outcome" || read().Status != "stuck" {
		t.Fatal("recovery lost post-landing failure")
	}
	if _, err = app.Submit(service.Command{ID: "resume-measurement", Action: "resume", UID: "p0"}); err != nil {
		t.Fatal(err)
	}
	task, err = app.Begin("p0")
	if err != nil {
		t.Fatal(err)
	}
	if task.Phase != "land-outcome" {
		t.Fatal("resume repeated the merge")
	}
	if _, err = app.Complete(task, workflow.Result{OK: true}); err != nil {
		t.Fatal(err)
	}
	var slot map[string]string
	if err = json.Unmarshal(app.Views.Read("", 0, []string{"slot/p0"}).Puts["slot/p0"], &slot); err != nil {
		t.Fatal(err)
	}
	if slot["proposal"] != "" || read().Status != "landed" {
		t.Fatal("completed measurement did not release the slot")
	}
}
