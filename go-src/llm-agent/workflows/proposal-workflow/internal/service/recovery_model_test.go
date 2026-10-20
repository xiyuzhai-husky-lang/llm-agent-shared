package service_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"proposal-platform/journal"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

// A model prefix with an unfinished external task must recover paused; a late
// result must not affect that state, while the original Start receipt survives.
func TestModelInterruptedTask(t *testing.T) {
	app, j, path := setup(t)
	task, err := app.Begin("p0")
	if err != nil {
		t.Fatal(err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	app, err = service.Open(j, testPolicies())
	if err != nil {
		t.Fatal(err)
	}
	before := app.Views.Read("", 0, []string{""}).Puts
	p := decodeProposal(t, before["proposal/p0"])
	if p.Status != "stopped" || p.Task != nil {
		t.Fatal("uncertain external work was automatically repeated")
	}
	if _, err = app.Complete(task, workflow.Result{OK: true}); err == nil {
		t.Fatal("stale result accepted")
	}
	if !reflect.DeepEqual(before, app.Views.Read("", 0, []string{""}).Puts) {
		t.Fatal("stale result changed state")
	}
	_, err = app.Submit(service.Command{ID: "start-p0", Action: "start", UID: "p0", Slot: "p0", Rank: "D"})
	if err != nil {
		t.Fatal("Start receipt lost in recovery", err)
	}
	if _, err = app.Submit(service.Command{ID: "resume", Action: "resume", UID: "p0"}); err != nil {
		t.Fatal(err)
	}
	next, err := app.Begin("p0")
	if err != nil || next.ID == task.ID {
		t.Fatal("resume did not create a distinct explicit operation")
	}
}

func decodeProposal(t *testing.T, data []byte) workflow.Proposal {
	t.Helper()
	var p workflow.Proposal
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}
