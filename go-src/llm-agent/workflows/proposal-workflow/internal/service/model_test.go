package service_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

func setup(tb testing.TB) (*service.Service, *journal.Journal, string) {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "workflow.sqlite")
	j, err := journal.Open(path)
	if err != nil {
		tb.Fatal(err)
	}
	app, err := service.Open(j, testPolicies())
	if err != nil {
		tb.Fatal(err)
	}
	commands := []service.Command{{ID: "batch", Action: "ledger", Ledger: &ledger.Action{Kind: "open", Batch: "b", Title: "Batch"}}, {ID: "approve", Action: "ledger", Ledger: &ledger.Action{Kind: "approve", Batch: "b", Source: "user"}}}
	for i := 0; i < 3; i++ {
		uid := fmt.Sprintf("p%d", i)
		commands = append(commands, service.Command{ID: "add-" + uid, Action: "ledger", Ledger: &ledger.Action{Kind: "add", Batch: "b", UID: uid}}, service.Command{ID: "start-" + uid, Action: "start", UID: uid, Slot: uid, Rank: "D"})
	}
	for _, command := range commands {
		if _, err = app.Submit(command); err != nil {
			tb.Fatal(err)
		}
	}
	return app, j, path
}

func TestRandomWorkflowModel(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			app, j, path := setup(t)
			defer func() { _ = j.Close() }()
			model := &referenceWorkflow{proposals: map[string]*referenceProposal{}, completed: map[string]string{}}
			for i := 0; i < 3; i++ {
				uid := fmt.Sprintf("p%d", i)
				model.proposals[uid] = &referenceProposal{uid: uid, status: "running", phase: "preflight", purpose: "initial", revision: 1, cap: 1}
			}
			random := rand.New(rand.NewSource(seed))
			epoch := ""
			cursor := uint64(0)
			cache := map[string]json.RawMessage{}
			executed := 0
			for step := 0; step < 500; step++ {
				uid := fmt.Sprintf("p%d", random.Intn(3))
				p := model.proposals[uid]
				before := app.Views.Read("", 0, []string{""}).Puts
				var receipt api.Receipt
				var err error
				if p.status == "running" && p.task == nil {
					want := model.begin(uid)
					task, beginErr := app.Begin(uid)
					if beginErr != nil {
						t.Fatal(beginErr)
					}
					if task.ID != want.ID || task.Phase != want.Phase || task.Purpose != want.Purpose {
						t.Fatal("task does not match model-generated operation")
					}
					// Task start and completion are separately checked declared transitions.
					start := app.Views.Read(epoch, cursor, []string{""})
					if start.Reset {
						cache = map[string]json.RawMessage{}
					}
					for key, value := range start.Puts {
						cache[key] = value
					}
					epoch = start.Epoch
					cursor = start.Cursor
					before = app.Views.Read("", 0, []string{""}).Puts
					result := workflow.Result{OK: random.Intn(14) != 0}
					if result.OK {
						switch p.phase {
						case "worker":
							if p.purpose == "feedback" && random.Intn(2) == 0 {
								result.Candidate = cloneCandidate(p.candidate)
								for _, d := range p.feedback {
									result.Answers = append(result.Answers, workflow.Answer{Doubt: d.ID, Disposition: "agreed", Reason: "acknowledged"})
								}
							} else {
								material := referenceCandidate(uid, fmt.Sprintf("r%d-op%d", p.revision, p.generation))
								if random.Intn(15) == 0 {
									material.Kind = "incomplete"
								}
								result.Candidate = &material
							}
						case "commissar", "reviewer", "nitpicker":
							if random.Intn(2) == 0 {
								result.Findings = []api.Doubt{{ID: fmt.Sprintf("d%d", p.generation), Text: "role finding"}}
							}
						case "response":
							for _, d := range p.feedback {
								result.Answers = append(result.Answers, workflow.Answer{Doubt: d.ID, Disposition: "agreed", Reason: "acknowledged"})
							}
							result.Changed = random.Intn(15) == 0
						case "wrapup":
							result.Candidate = cloneCandidate(p.candidate)
							result.Candidate.ID = fmt.Sprintf("wrapped-%d-%d", p.revision, p.generation)
						}
					}
					model.complete(want, result)
					receipt, err = app.Complete(task, result)
					if err == nil && random.Intn(4) == 0 {
						count := app.ViewCount()
						again, duplicateErr := app.Complete(task, result)
						if duplicateErr != nil || !reflect.DeepEqual(receipt, again) || app.ViewCount() != count {
							t.Fatal("duplicate task result had an effect")
						}
					}
				} else if p.status == "stopped" || p.status == "stuck" || p.status == "preflight-failed" || p.status == "delivery-failed" {
					p.status = "running"
					p.task = nil
					p.generation++
					receipt, err = app.Submit(service.Command{ID: fmt.Sprintf("resume-%d", step), Action: "resume", UID: uid})
				} else if p.candidate != nil && p.status != "aborted" {
					model.cursor++
					d := api.Decision{Sequence: model.cursor, ID: fmt.Sprintf("user-%d", step), Proposal: uid, Candidate: p.candidate.ID, Kind: "verdict", Signer: "Xiyu Zhai"}
					if p.verdict != "" {
						d.Value = "revoke"
					} else {
						d.Value = []string{"modify", "accept", "reject", "interact"}[random.Intn(4)]
					}
					if random.Intn(3) == 0 {
						d.Kind = "rule"
						d.Target = "d0"
						d.Value = "sustained"
					}
					model.decide(d)
					feed := api.Feed{Journal: "model", Cursor: model.cursor, Decisions: []api.Decision{d}}
					receipt, err = app.Consume(feed)
					if err == nil {
						count := app.ViewCount()
						again, duplicateErr := app.Consume(feed)
						if duplicateErr != nil || !reflect.DeepEqual(receipt, again) || app.ViewCount() != count {
							t.Fatal("feed replay changed state")
						}
					}
				} else {
					continue
				}
				if err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				executed++
				after := app.Views.Read("", 0, []string{""}).Puts
				keys := map[string]bool{}
				for _, key := range receipt.Keys {
					keys[key] = true
				}
				for key, value := range after {
					if !reflect.DeepEqual(before[key], value) && !keys[key] {
						t.Fatalf("undeclared key changed: %s", key)
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
					t.Fatal("workflow client cache diverged")
				}
				if err = model.check(after); err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				if step%83 == 82 {
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
					if !reflect.DeepEqual(after, app.Views.Read("", 0, []string{""}).Puts) {
						t.Fatal("recovery diverged from acknowledged state")
					}
				}
			}
			t.Logf("seed=%d compared transitions=%d", seed, executed)
		})
	}
}
