package service_test

import (
	"context"
	"path/filepath"
	"testing"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/journal"
)

func TestIdentityRecoveryModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.sqlite")
	j, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app, err := service.Open(j, &candidate.Builder{}, "First Signer")
	if err != nil {
		t.Fatal(err)
	}
	material := fixture("p", "r1", 0)
	if _, err = app.Deliver(context.Background(), api.Delivery{ID: "candidate", Candidate: material}); err != nil {
		t.Fatal(err)
	}
	first := service.Command{ID: "first", Proposal: "p", Candidate: "r1", Action: "sign", Target: material.Hunks[0].ID}
	if _, err = app.Submit(first); err != nil {
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
	app, err = service.Open(j, &candidate.Builder{}, "Second Signer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Submit(first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "second"
	second.Target = material.Hunks[1].ID
	if _, err = app.Submit(second); err != nil {
		t.Fatal(err)
	}
	frame := app.Views.Read("", 0, []string{"hunk/"})
	for i, want := range []string{"First Signer", "Second Signer"} {
		h := decode[struct {
			Signature struct {
				Signer string `json:"signer"`
			} `json:"signature"`
		}](frame.Puts["hunk/p/r1/"+material.Hunks[i].ID])
		if h.Signature.Signer != want {
			t.Fatalf("signature %d belongs to %q, expected %q", i, h.Signature.Signer, want)
		}
	}
	if _, err = service.Open(j, &candidate.Builder{}, "   "); err == nil {
		t.Fatal("empty signer accepted")
	}
}
