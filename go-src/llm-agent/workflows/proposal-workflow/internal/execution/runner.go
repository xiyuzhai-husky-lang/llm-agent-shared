package execution

import (
	"context"
	"proposal-workflow/internal/workflow"
)

// Runner executes one task; only the service can commit its result.
type Runner interface {
	Run(context.Context, workflow.Task) (workflow.Result, error)
}
