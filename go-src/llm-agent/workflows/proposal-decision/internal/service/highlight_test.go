package service_test

import (
	"context"
	"path/filepath"
	"proposal-decision/internal/candidate"
	"proposal-decision/internal/highlight"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/journal"
	"strings"
	"testing"
)

func TestHighlightIsBuiltOnceAndReplayed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.sqlite")
	j, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	highlighter := &highlight.Builder{}
	builder := &candidate.Builder{Highlighter: highlighter}
	app, err := service.Open(j, builder, "Scratch Reviewer")
	if err != nil {
		t.Fatal(err)
	}
	h := api.Hunk{Path: "source.go", Header: "@@ -1 +1 @@", Before: "var n = 1\n", After: "var n = 2\n", MustSign: true}
	h.ID = candidate.HunkID(h)
	input := api.Delivery{ID: "deliver", Candidate: api.Candidate{Proposal: "p", ID: "r1", Kind: "complete", Hunks: []api.Hunk{h}, Doubts: []api.Doubt{{ID: "d", Text: "Review this change", Hunk: h.ID}}}}
	if _, err = app.Deliver(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	saved, ok := app.PeekMaterial("p", "r1")
	if !ok || saved.Hunks[0].ID != h.ID || !strings.Contains(saved.Hunks[0].Highlight.After, `syntax-keyword`) {
		t.Fatalf("missing highlighted material: %+v", saved)
	}
	if highlighter.Parses.Load() != 2 {
		t.Fatal("expected two parses at delivery")
	}
	if _, err = app.Deliver(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	for _, command := range []service.Command{
		{ID: "sign", Proposal: "p", Candidate: "r1", Action: "sign", Target: h.ID},
		{ID: "rule", Proposal: "p", Candidate: "r1", Action: "rule", Target: "d", Value: "sustained"},
	} {
		if _, err = app.Submit(command); err != nil {
			t.Fatal(err)
		}
	}
	if highlighter.Parses.Load() != 2 || highlighter.GitCalls.Load() != 0 {
		t.Fatal("decision recomputed highlighting")
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	replayHighlighter := &highlight.Builder{}
	replay, err := service.Open(j, &candidate.Builder{Highlighter: replayHighlighter}, "Scratch Reviewer")
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := replay.PeekMaterial("p", "r1")
	if !ok || *restored.Hunks[0].Highlight != *saved.Hunks[0].Highlight || replayHighlighter.Parses.Load() != 0 || replayHighlighter.GitCalls.Load() != 0 {
		t.Fatal("replay rebuilt or changed syntax")
	}
}
