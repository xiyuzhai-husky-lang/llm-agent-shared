package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"proposal-decision/internal/candidate"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/journal"
)

func TestRandomModel(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			random := rand.New(rand.NewSource(seed))
			path := filepath.Join(t.TempDir(), "decision.sqlite")
			j, err := journal.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			builder := &candidate.Builder{}
			app, err := service.Open(j, builder, "Xiyu Zhai")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = j.Close() }()
			model := newReference()
			cache := map[string]json.RawMessage{}
			epoch := ""
			cursor := uint64(0)
			saved := []service.Command{}
			receipts := map[string]api.Receipt{}
			revision := 0
			for step := 0; step < 350; step++ {
				before := app.Views.Read("", 0, []string{""}).Puts
				var receipt api.Receipt
				var accepted bool
				var command *service.Command
				allowed := map[string]bool{}
				if step < 2 || random.Intn(8) == 0 {
					revision++
					material := fixture(fmt.Sprintf("p%d", revision%2), fmt.Sprintf("r%d", revision), random.Intn(9))
					delivery := api.Delivery{ID: fmt.Sprintf("delivery-%d", revision), Candidate: material}
					previous := model.heads[material.Proposal]
					for _, key := range []string{"proposal/" + material.Proposal, "candidate/" + material.Proposal + "/" + material.ID, "revision/" + material.Proposal + "/" + material.ID, "revision/" + material.Proposal + "/" + previous} {
						allowed[key] = true
					}
					for _, h := range material.Hunks {
						allowed["hunk/"+material.Proposal+"/"+material.ID+"/"+h.ID] = true
					}
					for _, d := range material.Doubts {
						allowed["doubt/"+material.Proposal+"/"+material.ID+"/"+d.ID] = true
					}
					for _, d := range material.Documents {
						allowed["document/"+material.Proposal+"/"+material.ID+"/"+d.ID] = true
					}
					accepted = model.deliver(delivery)
					receipt, err = app.Deliver(context.Background(), delivery)
					if accepted && random.Intn(3) == 0 {
						cost := app.Cost()
						again, repeatErr := app.Deliver(context.Background(), delivery)
						if repeatErr != nil || !reflect.DeepEqual(receipt, again) || app.Cost() != cost {
							t.Fatalf("delivery retry: err=%v before=%+v after=%+v receipt=%+v again=%+v", repeatErr, cost, app.Cost(), receipt, again)
						}
					}
				} else {
					keys := []string{}
					for key := range model.revisions {
						keys = append(keys, key)
					}
					// Sort before drawing so every seed has a reproducible command sequence.
					sortStrings(keys)
					r := model.revisions[keys[random.Intn(len(keys))]]
					c := service.Command{ID: fmt.Sprintf("command-%d", step), Proposal: r.material.Proposal, Candidate: r.material.ID}
					switch random.Intn(7) {
					case 0:
						c.Action = "open_diff"
					case 1:
						c.Action = "sign"
						c.Target = r.material.Hunks[random.Intn(3)].ID
					case 2:
						c.Action = "sign_under"
					case 3:
						c.Action = "rule"
						c.Target = fmt.Sprintf("d%d", random.Intn(2))
						c.Value = []string{"sustained", "dismissed", "followup", "defer", "revoked"}[random.Intn(5)]
					case 4, 5:
						c.Action = "verdict"
						c.Value = []string{"accept", "modify", "reject", "interact", "revoke"}[random.Intn(5)]
						c.KeepSignatures = c.Value == "modify" && random.Intn(2) == 0
					case 6:
						c.Action = "comment"
						c.Text = "A user comment"
					}
					repeat := len(saved) > 0 && random.Intn(7) == 0
					if repeat {
						c = saved[random.Intn(len(saved))]
					}
					command = &c
					scope := c.Proposal + "/" + c.Candidate
					allowed["entry/"+c.Proposal+"/"+c.ID] = true
					if c.Action != "comment" {
						allowed["revision/"+scope] = true
					}
					if c.Action == "sign" {
						allowed["hunk/"+scope+"/"+c.Target] = true
					}
					if c.Action == "sign_under" {
						for _, h := range model.revisions[refKey(c.Proposal, c.Candidate)].material.Hunks {
							if h.MustSign {
								allowed["hunk/"+scope+"/"+h.ID] = true
							}
						}
					}
					if c.Action == "rule" {
						allowed["doubt/"+scope+"/"+c.Target] = true
					}
					priorGate := model.gate(r)
					accepted = model.command(c)
					cost := app.Cost()
					receipt, err = app.Submit(c)
					if accepted {
						if repeat {
							if !reflect.DeepEqual(receipts[c.ID], receipt) || !reflect.DeepEqual(before, app.Views.Read("", 0, []string{""}).Puts) || app.Cost() != cost {
								t.Fatalf("repeated command: err=%v before=%+v after=%+v receipt=%+v original=%+v", err, cost, app.Cost(), receipt, receipts[c.ID])
							}
						} else {
							saved = append(saved, c)
							receipts[c.ID] = receipt
						}
						if !repeat && priorGate && (c.Action == "sign" || c.Action == "sign_under" || c.Action == "open_diff" || c.Action == "rule" && c.Value != "revoked") && !model.gate(r) {
							t.Fatal("gate was not monotone under added approvals")
						}
					}
				}
				if (err == nil) != accepted {
					t.Fatalf("step %d command %+v accepted=%v error=%v", step, command, accepted, err)
				}
				after := app.Views.Read("", 0, []string{""}).Puts
				declared := map[string]bool{}
				if accepted {
					for _, key := range receipt.Keys {
						if !allowed[key] {
							t.Fatalf("command declared unrelated key %s", key)
						}
						declared[key] = true
					}
				}
				for key, value := range after {
					if !reflect.DeepEqual(before[key], value) && !declared[key] {
						t.Fatalf("undeclared key changed: %s", key)
					}
				}
				for key := range before {
					if _, ok := after[key]; !ok && !declared[key] {
						t.Fatal("undeclared deletion")
					}
				}
				frame := app.Views.Read(epoch, cursor, []string{""})
				if frame.Reset {
					cache = map[string]json.RawMessage{}
				}
				for key, value := range frame.Puts {
					cache[key] = value
				}
				epoch = frame.Epoch
				cursor = frame.Cursor
				if !reflect.DeepEqual(cache, after) {
					t.Fatal("client cache diverged")
				}
				if err = model.check(after); err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				if step%71 == 70 {
					if err = j.Close(); err != nil {
						t.Fatal(err)
					}
					j, err = journal.Open(path)
					if err != nil {
						t.Fatal(err)
					}
					builder = &candidate.Builder{}
					app, err = service.Open(j, builder, "Xiyu Zhai")
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(after, app.Views.Read("", 0, []string{""}).Puts) || builder.Builds.Load() != 0 {
						t.Fatal("recovery changed acknowledged state or rebuilt candidates")
					}
				}
			}
		})
	}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
