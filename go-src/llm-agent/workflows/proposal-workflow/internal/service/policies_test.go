package service_test

import "proposal-workflow/internal/workflow"

func testPolicies() map[string]workflow.Policy {
	result := map[string]workflow.Policy{}
	for i, name := range []string{"D", "C", "B", "A", "S", "SS", "SSS"} {
		result[name] = workflow.Policy{Rank: name, WorkerRevisions: i + 1, GateRepairs: 2, WrapRepairs: 2, DeadlineSeconds: 900 << i}
	}
	return result
}
