package service_test

import (
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/service"
	"testing"
)

// A physical directory cannot be assigned twice, including while its owner is stopped.
// Settlement releases it; an acknowledged Start remains idempotent afterwards.
func TestDirectoryReservationModel(t *testing.T) {
	app, j, _ := setup(t)
	defer j.Close()
	for _, uid := range []string{"first", "second"} {
		if _, err := app.Submit(service.Command{ID: "add-" + uid, Action: "ledger", Ledger: &ledger.Action{Kind: "add", Batch: "b", UID: uid}}); err != nil {
			t.Fatal(err)
		}
	}
	first := service.Command{ID: "first-start", Action: "start", UID: "first", Slot: "first", Directory: "/slot", Rank: "D"}
	second := service.Command{ID: "second-start", Action: "start", UID: "second", Slot: "second", Directory: "/slot", Rank: "D"}
	if _, err := app.Submit(first); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Submit(second); err == nil {
		t.Fatal("same directory admitted twice")
	}
	if _, err := app.Submit(service.Command{ID: "stop-first", Action: "stop", UID: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Submit(second); err == nil {
		t.Fatal("stopping released the directory")
	}
	if _, err := app.Submit(service.Command{ID: "settle-first", Action: "ledger", Ledger: &ledger.Action{Kind: "settle", UID: "first", Value: "abandoned", Source: "test"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Submit(second); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Submit(first); err != nil {
		t.Fatal("original start receipt was lost", err)
	}
}
