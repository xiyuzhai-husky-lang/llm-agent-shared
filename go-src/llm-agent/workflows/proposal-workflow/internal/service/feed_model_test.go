package service_test

import (
	"proposal-platform/api"
	"proposal-workflow/internal/service"
	"testing"
)

func TestIdleFeedDoesNotReplayLastPage(t *testing.T) {
	app, j, _ := setup(t)
	defer j.Close()
	page := api.Feed{Journal: "first", Cursor: 1, Decisions: []api.Decision{{Sequence: 1, Kind: "comment", Proposal: "p0", Text: "comment"}}}
	if _, err := app.Consume(page); err != nil {
		t.Fatal(err)
	}
	count := app.ViewCount()
	for i := 0; i < 10; i++ {
		if _, err := app.Consume(api.Feed{Journal: "first", Cursor: 1, Decisions: []api.Decision{}}); err != nil {
			t.Fatal(err)
		}
	}
	if app.ViewCount() != count {
		t.Fatal("idle heartbeat recomputed views")
	}
	if _, err := app.Consume(page); err != nil {
		t.Fatal("retry of the original page lost its receipt", err)
	}
	if _, err := app.Consume(api.Feed{Journal: "second", Cursor: 0}); err == nil {
		t.Fatal("silently reconnected a changed journal")
	}
	if _, err := app.Submit(service.Command{ID: "connect", Action: "connect_decisions", Journal: "second", Source: "operator replaced empty decision journal"}); err != nil {
		t.Fatal(err)
	}
	journal, cursor := app.Checkpoint()
	if journal != "second" || cursor != 0 {
		t.Fatal("explicit connection did not reset checkpoint")
	}
	if _, err := app.Submit(service.Command{ID: "reuse-old-journal", Action: "connect_decisions", Journal: "first", Source: "test"}); err == nil {
		t.Fatal("reconnecting a previously consumed journal would reuse old page receipts")
	}
}
