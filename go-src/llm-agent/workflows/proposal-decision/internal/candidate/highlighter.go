package candidate

import (
	"context"
	"proposal-platform/api"
)

// Highlighter constructs presentation once, before a candidate is journaled.
type Highlighter interface {
	Highlight(context.Context, *api.Source, []api.Hunk) error
}
