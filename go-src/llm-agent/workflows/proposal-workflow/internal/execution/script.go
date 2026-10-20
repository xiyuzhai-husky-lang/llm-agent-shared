package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"proposal-workflow/internal/service"
	"sync"

	"proposal-workflow/internal/workflow"
)

// Script owns immutable results keyed by proposal/revision/phase/purpose or phase.
type Script struct {
	Results map[string]workflow.Result `json:"results"`
}

func LoadScript(path string) (*Script, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var script Script
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&script); err != nil {
		return nil, err
	}
	if script.Results == nil {
		return nil, fmt.Errorf("scenario requires results")
	}
	return &script, nil
}

func (s *Script) Run(ctx context.Context, task workflow.Task) (workflow.Result, error) {
	if err := ctx.Err(); err != nil {
		return workflow.Result{}, err
	}
	keys := []string{task.ID, fmt.Sprintf("%s/%d/%s/%s", task.Proposal, task.Revision, task.Phase, task.Purpose), task.Phase}
	for _, key := range keys {
		if result, ok := s.Results[key]; ok {
			data, err := json.Marshal(result)
			if err != nil {
				return result, err
			}
			var copy workflow.Result
			err = json.Unmarshal(data, &copy)
			return copy, err
		}
	}
	return workflow.Result{}, fmt.Errorf("script has no result for %s", keys[1])
}

// Stepper advances only after an explicit tick; no process or timer runs a scenario.
type Stepper struct {
	Coordinator *Coordinator
	Service     *service.Service
	mu          sync.Mutex
}

func (s *Stepper) Tick(ctx context.Context, n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < n; i++ {
		for _, uid := range s.Service.Ready() {
			task, err := s.Service.Begin(uid)
			if err != nil {
				return err
			}
			result, err := s.Coordinator.execute(ctx, task)
			if err != nil {
				result = workflow.Result{OK: false, Detail: err.Error()}
			}
			if _, err := s.Service.Complete(task, result); err != nil {
				return err
			}
		}
	}
	return nil
}
