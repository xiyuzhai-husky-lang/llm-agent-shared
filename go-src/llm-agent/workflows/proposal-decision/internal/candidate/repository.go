package candidate

import (
	"context"
	"proposal-platform/api"
)

// Repository supplies a read-only diff of a pinned source range.
type Repository interface {
	Patch(context.Context, api.Source) (string, error)
}
