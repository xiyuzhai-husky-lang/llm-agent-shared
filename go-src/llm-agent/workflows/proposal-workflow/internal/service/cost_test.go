package service_test

import (
	"fmt"
	"testing"

	"proposal-platform/api"
)

func TestDecisionFeedCost(t *testing.T) {
	for _, history := range []int{1, 1000} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			app, j, _ := setup(t)
			defer j.Close()
			for i := 1; i <= history; i++ {
				_, err := app.Consume(api.Feed{Journal: "cost", Cursor: uint64(i), Decisions: []api.Decision{{Sequence: uint64(i), Proposal: "p0", Candidate: fmt.Sprint(i), Kind: "sign"}}})
				if err != nil {
					t.Fatal(err)
				}
			}
			before := app.ViewCount()
			_, err := app.Consume(api.Feed{Journal: "cost", Cursor: uint64(history + 1), Decisions: []api.Decision{{Sequence: uint64(history + 1), Proposal: "p0", Candidate: "latest", Kind: "rule", Value: "sustained"}}})
			if err != nil {
				t.Fatal(err)
			}
			if app.ViewCount()-before != 2 {
				t.Fatal("decision consumption grew with history")
			}
			t.Logf("history=%d feed views=2; no runner or Git capability in consumption", history)
		})
	}
}

func BenchmarkDecisionFeed(b *testing.B) {
	for _, history := range []int{1, 1000} {
		b.Run(fmt.Sprint(history), func(b *testing.B) {
			app, j, _ := setup(b)
			defer j.Close()
			for i := 1; i <= history; i++ {
				_, err := app.Consume(api.Feed{Journal: "bench", Cursor: uint64(i), Decisions: []api.Decision{{Sequence: uint64(i), Proposal: "p0", Kind: "sign"}}})
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sequence := uint64(history + i + 1)
				_, err := app.Consume(api.Feed{Journal: "bench", Cursor: sequence, Decisions: []api.Decision{{Sequence: sequence, Proposal: "p0", Kind: "rule", Value: "sustained"}}})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
