package service_test

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"proposal-workflow/internal/ledger"
)

// TestRandomLedgerModel generates operations from a small flat membership model.
func TestRandomLedgerModel(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		random := rand.New(rand.NewSource(seed))
		actual := ledger.New()
		batches := map[string]bool{}
		approved := map[string]bool{}
		members := map[string]string{}
		settled := map[string]bool{}
		stack := []string{}
		for step := 0; step < 200; step++ {
			id := fmt.Sprintf("b%d", random.Intn(8))
			uid := fmt.Sprintf("p%d", random.Intn(20))
			a := ledger.Action{Batch: id, UID: uid, Source: "user"}
			allowed := false
			switch random.Intn(6) {
			case 0:
				a.Kind = "open"
				_, exists := batches[id]
				allowed = !exists
				if allowed {
					batches[id] = false
					stack = append([]string{id}, stack...)
				}
			case 1:
				a.Kind = "add"
				closed, exists := batches[id]
				allowed = exists && !closed && members[uid] == ""
				if allowed {
					members[uid] = id
				}
			case 2:
				a.Kind = "approve"
				closed, exists := batches[id]
				allowed = exists && !closed
				if allowed {
					approved[id] = true
				}
			case 3:
				a.Kind = "withdraw"
				allowed = members[uid] != "" && !settled[uid]
				if allowed {
					holder := members[uid]
					delete(members, uid)
					delete(settled, uid)
					empty := true
					for _, batch := range members {
						if batch == holder {
							empty = false
						}
					}
					if empty {
						delete(batches, holder)
						delete(approved, holder)
						stack = slices.DeleteFunc(stack, func(batch string) bool { return batch == holder })
					}
				}
			case 4:
				a.Kind = "transfer"
				closed, exists := batches[id]
				allowed = exists && !closed && members[uid] != "" && !settled[uid] && members[uid] != id
				if allowed {
					members[uid] = id
				}
			case 5:
				a.Kind = "close"
				allowed = len(stack) > 0 && stack[0] == id
				for member, batch := range members {
					if batch == id && !settled[member] {
						allowed = false
					}
				}
				if allowed {
					batches[id] = true
					stack = stack[1:]
				}
			}
			next, err := actual.Plan(a)
			if (err == nil) != allowed {
				t.Fatalf("seed=%d step=%d action=%+v allowed=%v err=%v", seed, step, a, allowed, err)
			}
			if allowed {
				actual = next
			}
			if !slices.Equal(actual.Stack, stack) {
				t.Fatalf("seed=%d step=%d: stack diverged: got %v, want %v", seed, step, actual.Stack, stack)
			}
			if len(actual.Members) != len(members) {
				t.Fatal("membership count diverged")
			}
			for member, batch := range members {
				if actual.Members[member] == nil || actual.Members[member].Batch != batch || (actual.Members[member].Settlement != "") != settled[member] {
					t.Fatal("membership diverged")
				}
			}
			for member, batch := range members {
				eligible := len(stack) > 0 && stack[0] == batch && !settled[member] && approved[batch] && !batches[batch]
				if (actual.CanStart(member) == nil) != eligible {
					t.Fatal("batch gate diverged")
				}
			}
		}
	}
}
