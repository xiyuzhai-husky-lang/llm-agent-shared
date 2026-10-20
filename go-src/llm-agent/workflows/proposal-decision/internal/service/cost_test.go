package service_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/journal"
)

func populated(tb testing.TB, revisions int) (*service.Service, *journal.Journal, api.Candidate) {
	tb.Helper()
	j, err := journal.Open(filepath.Join(tb.TempDir(), "cost.sqlite"))
	if err != nil {
		tb.Fatal(err)
	}
	app, err := service.Open(j, &candidate.Builder{}, "Xiyu Zhai")
	if err != nil {
		tb.Fatal(err)
	}
	var material api.Candidate
	for i := 0; i < revisions; i++ {
		material = fixture("p", fmt.Sprintf("r%d", i), 0)
		before := app.Cost()
		_, err = app.Deliver(context.Background(), api.Delivery{ID: material.ID, Candidate: material})
		if err != nil {
			tb.Fatal(err)
		}
		if app.Cost().Builds-before.Builds != 1 {
			tb.Fatal("delivery did not build exactly once")
		}
	}
	return app, j, material
}

func TestOperationBounds(t *testing.T) {
	for _, history := range []int{1, 10, 100, 1000} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			app, j, material := populated(t, history)
			defer j.Close()
			before := app.Cost()
			if _, err := app.Submit(service.Command{ID: "sign", Proposal: "p", Candidate: material.ID, Action: "sign", Target: material.Hunks[0].ID}); err != nil {
				t.Fatal(err)
			}
			signed := app.Cost()
			if _, err := app.Submit(service.Command{ID: "sustain", Proposal: "p", Candidate: material.ID, Action: "rule", Target: "d0", Value: "sustained"}); err != nil {
				t.Fatal(err)
			}
			sustained := app.Cost()
			if signed.Builds != before.Builds || signed.GitCalls != before.GitCalls || sustained.Builds != before.Builds || sustained.GitCalls != before.GitCalls {
				t.Fatal("user action performed heavy work")
			}
			if signed.Views-before.Views != 3 || sustained.Views-signed.Views != 3 || signed.Visits-before.Visits > 5 || sustained.Visits != signed.Visits {
				t.Fatalf("cost depends on history: %+v %+v %+v", before, signed, sustained)
			}
			t.Logf("revisions=%d Sign: git=0 builds=0 views=%d visits=%d; Sustain: git=0 builds=0 views=%d visits=0", history, signed.Views-before.Views, signed.Visits-before.Visits, sustained.Views-signed.Views)
		})
	}
}

func BenchmarkSign(b *testing.B) {
	for _, history := range []int{1, 1000} {
		b.Run(fmt.Sprint(history), func(b *testing.B) {
			app, j, material := populated(b, history)
			defer j.Close()
			b.ResetTimer()
			// Fresh candidate setup is excluded so every timed Sign is a real mutation.
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				material.ID = fmt.Sprintf("bench%d", i)
				_, err := app.Deliver(context.Background(), api.Delivery{ID: material.ID, Candidate: material})
				if err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				if _, err = app.Submit(service.Command{ID: fmt.Sprintf("s%d", i), Proposal: "p", Candidate: material.ID, Action: "sign", Target: material.Hunks[0].ID}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSustain(b *testing.B) {
	for _, history := range []int{1, 1000} {
		b.Run(fmt.Sprint(history), func(b *testing.B) {
			app, j, material := populated(b, history)
			defer j.Close()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				_, err := app.Submit(service.Command{ID: fmt.Sprintf("clear%d", i), Proposal: "p", Candidate: material.ID, Action: "rule", Target: "d0", Value: "revoked"})
				if err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				if _, err = app.Submit(service.Command{ID: fmt.Sprintf("rule%d", i), Proposal: "p", Candidate: material.ID, Action: "rule", Target: "d0", Value: "sustained"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestGitCapabilityCost(t *testing.T) {
	j, err := journal.Open(filepath.Join(t.TempDir(), "git.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	repository := &countedRepository{}
	builder := &candidate.Builder{Repository: repository}
	app, err := service.Open(j, builder, "Xiyu Zhai")
	if err != nil {
		t.Fatal(err)
	}
	material := api.Candidate{Proposal: "p", ID: "1111111111111111111111111111111111111111", Base: "0000000000000000000000000000000000000000", Kind: "complete", Source: &api.Source{Repository: "opaque", Base: "0000000000000000000000000000000000000000", Commit: "1111111111111111111111111111111111111111", MustSign: []string{"code.go"}}, Doubts: []api.Doubt{{ID: "d0", Text: "Doubt"}}}
	delivery := api.Delivery{ID: "source", Candidate: material}
	if _, err = app.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || builder.Builds.Load() != 1 {
		t.Fatal("delivery/retry did not build and read Git exactly once")
	}
	target := ""
	for key := range app.Views.Read("", 0, []string{"hunk/"}).Puts {
		target = key[len("hunk/p/"+material.ID+"/"):]
	}
	if _, err = app.Submit(service.Command{ID: "sign-git", Proposal: "p", Candidate: material.ID, Action: "sign", Target: target}); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Submit(service.Command{ID: "rule-git", Proposal: "p", Candidate: material.ID, Action: "rule", Target: "d0", Value: "sustained"}); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || builder.Builds.Load() != 1 {
		t.Fatal("Sign/Sustain reached the Git capability or rebuilt a candidate")
	}
}

func BenchmarkDelivery(b *testing.B) {
	for _, history := range []int{1, 1000} {
		b.Run(fmt.Sprint(history), func(b *testing.B) {
			app, j, material := populated(b, history)
			defer j.Close()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				material.ID = fmt.Sprintf("delivery-bench%d", i)
				if _, err := app.Deliver(context.Background(), api.Delivery{ID: material.ID, Candidate: material}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
