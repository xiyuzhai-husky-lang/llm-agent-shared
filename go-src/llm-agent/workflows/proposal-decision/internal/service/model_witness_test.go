package service_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/journal"
)

// This model-generated witness reaches carry, revocation and Accept explicitly;
// each acknowledged prefix is reopened to check durable replay and idempotency.
func TestModelCarryAndRevocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefix.sqlite")
	j, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err := service.Open(j, &candidate.Builder{}, "Xiyu Zhai")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = j.Close() }()
	model := newReference()
	check := func() {
		t.Helper()
		before := app.Views.Read("", 0, []string{""})
		if err := model.check(before.Puts); err != nil {
			t.Fatal(err)
		}
		if err := j.Close(); err != nil {
			t.Fatal(err)
		}
		j, err = journal.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		builder := &candidate.Builder{}
		app, err = service.Open(j, builder, "Xiyu Zhai")
		if err != nil {
			t.Fatal(err)
		}
		after := app.Views.Read(before.Epoch, before.Cursor, []string{""})
		if !after.Reset || !reflect.DeepEqual(before.Puts, after.Puts) || builder.Builds.Load() != 0 {
			t.Fatal("prefix recovery mismatch")
		}
	}
	first := fixture("p", "r1", 0)
	delivery := api.Delivery{ID: "r1", Candidate: first}
	if !model.deliver(delivery) {
		t.Fatal("oracle delivery")
	}
	if _, err = app.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	check()
	commands := []service.Command{
		{Action: "open_diff"}, {Action: "sign", Target: first.Hunks[0].ID},
		{Action: "rule", Target: "d0", Value: "sustained"}, {Action: "rule", Target: "d1", Value: "defer"},
		{Action: "sign", Target: first.Hunks[1].ID}, {Action: "verdict", Value: "accept"},
		{Action: "rule", Target: "d0", Value: "revoked"}, {Action: "verdict", Value: "revoke"},
		{Action: "verdict", Value: "accept"}, // Must fail after the ruling was revoked.
		{Action: "rule", Target: "d0", Value: "dismissed"}, {Action: "verdict", Value: "modify", KeepSignatures: true},
	}
	for i, c := range commands {
		c.ID = fmt.Sprintf("witness%d", i)
		c.Proposal = "p"
		c.Candidate = "r1"
		accepted := model.command(c)
		receipt, err := app.Submit(c)
		if (err == nil) != accepted {
			t.Fatalf("%+v: %v", c, err)
		}
		if i == 1 && receipt.Next != "hunk/p/r1/"+first.Hunks[1].ID {
			t.Fatal("Sign stayed at a hunk with unresolved Doubts")
		}
		check()
		if accepted {
			before := app.Cost()
			again, err := app.Submit(c)
			if err != nil || !reflect.DeepEqual(receipt, again) || app.Cost() != before {
				t.Fatal("recovered command was not idempotent")
			}
		}
	}
	second := fixture("p", "r2", 1)
	delivery = api.Delivery{ID: "r2", Candidate: second}
	if !model.deliver(delivery) {
		t.Fatal("oracle carry delivery")
	}
	if _, err = app.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	check()
	if len(model.revisions["p/r2"].signatures) != 1 {
		t.Fatal("witness did not reach selective signature carry")
	}
	revoke := service.Command{ID: "late-revoke", Proposal: "p", Candidate: "r1", Action: "verdict", Value: "revoke"}
	if !model.command(revoke) {
		t.Fatal("oracle late revoke")
	}
	if _, err = app.Submit(revoke); err != nil {
		t.Fatal(err)
	}
	check()
	conflict := commands[1]
	conflict.ID = "witness1"
	conflict.Proposal = "p"
	conflict.Candidate = "r1"
	conflict.Target = first.Hunks[1].ID
	before := app.Views.Read("", 0, []string{""}).Puts
	if _, err = app.Submit(conflict); err == nil {
		t.Fatal("conflicting command ID accepted")
	}
	if !reflect.DeepEqual(before, app.Views.Read("", 0, []string{""}).Puts) {
		t.Fatal("conflicting retry changed state")
	}
	feed, err := app.Feed(context.Background(), "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range feed.Decisions {
		if d.Signer != "Xiyu Zhai" {
			t.Fatal("wrong permanent signer")
		}
		if d.Kind == "worker" {
			t.Fatal("Worker reply entered decision feed")
		}
	}
}
