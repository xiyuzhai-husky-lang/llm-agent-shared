package execution

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"proposal-platform/api"
	"proposal-workflow/internal/decisionclient"
	"proposal-workflow/internal/workflow"
)

func TestInvalidDeliveryDoesNotRebuildCandidate(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "anchor does not match candidate", http.StatusBadRequest)
	}))
	defer server.Close()
	c := Coordinator{Decision: &decisionclient.Client{URL: server.URL, HTTP: server.Client()}}
	_, err := c.execute(context.Background(), workflow.Task{Phase: "deliver", Candidate: &api.Candidate{Proposal: "test", ID: "candidate"}})
	var refusal *decisionclient.ResponseError
	if !errors.As(err, &refusal) || refusal.Status != http.StatusBadRequest {
		t.Fatalf("expected delivery refusal, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("invalid candidate was sent %d times", got)
	}
}
