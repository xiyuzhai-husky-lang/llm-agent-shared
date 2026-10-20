package execution

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"proposal-workflow/internal/decisionclient"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

// Coordinator owns active task cancellation and drives only event-announced work.
type Coordinator struct {
	Service  *service.Service
	Runner   Runner
	Decision *decisionclient.Client
}

func (c *Coordinator) Run(ctx context.Context) {
	tasks := map[string]workflow.Task{}
	directories := map[string]*sync.Mutex{}
	cancels := map[string]context.CancelFunc{}
	finished := make(chan string, 32)
	var group sync.WaitGroup
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
		group.Wait()
	}()
	for {
		for uid, task := range tasks {
			if !c.Service.Active(task) {
				cancels[uid]()
			}
		}
		for _, uid := range c.Service.Ready() {
			if _, exists := tasks[uid]; exists {
				continue
			}
			task, err := c.Service.Begin(uid)
			if err != nil {
				continue
			}
			lock := directories[task.Directory]
			if lock == nil {
				lock = &sync.Mutex{}
				directories[task.Directory] = lock
			}
			runCtx, cancel := context.WithCancel(ctx)
			tasks[uid] = task
			cancels[uid] = cancel
			group.Add(1)
			go func(task workflow.Task) {
				defer group.Done()
				lock.Lock()
				var result workflow.Result
				err := runCtx.Err()
				if err == nil {
					result, err = c.execute(runCtx, task)
				}
				lock.Unlock()
				if err != nil {
					result = workflow.Result{OK: false, Detail: err.Error()}
				}
				if _, err = c.Service.Complete(task, result); err != nil && runCtx.Err() == nil {
					log.Printf("complete %s: %v", task.ID, err)
				}
				select {
				case finished <- task.Proposal:
				case <-ctx.Done():
				}
			}(task)
		}
		select {
		case <-ctx.Done():
			return
		case uid := <-finished:
			cancels[uid]()
			delete(cancels, uid)
			delete(tasks, uid)
		case <-c.Service.Wake:
		}
	}
}

func (c *Coordinator) execute(ctx context.Context, task workflow.Task) (workflow.Result, error) {
	if task.Phase != "deliver" {
		return c.Runner.Run(ctx, task)
	}
	if task.Candidate == nil {
		return workflow.Result{OK: false, Detail: "delivery has no candidate"}, nil
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		_, err = c.Decision.Deliver(ctx, *task.Candidate)
		if err == nil {
			return workflow.Result{OK: true}, nil
		}
		var refusal *decisionclient.ResponseError
		if errors.As(err, &refusal) && refusal.Status >= 400 && refusal.Status < 500 && refusal.Status != 429 {
			return workflow.Result{}, err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return workflow.Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	return workflow.Result{}, err
}

func (c *Coordinator) Subscribe(ctx context.Context) {
	for ctx.Err() == nil {
		identity, cursor := c.Service.Checkpoint()
		feed, err := c.Decision.Feed(ctx, identity, cursor)
		if err == nil {
			_, err = c.Service.Consume(feed)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("decision feed: %v", err)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}
