package service_test

import (
	"context"
	"proposal-platform/api"
)

// countedRepository records actual invocations of the delivery-only Git capability.
type countedRepository struct{ calls int }

func (r *countedRepository) Patch(context.Context, api.Source) (string, error) {
	r.calls++
	return "diff --git a/code.go b/code.go\n--- a/code.go\n+++ b/code.go\n@@ -1 +1 @@\n-old\n+new\n", nil
}
