package cli

import (
	"context"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/repository"
)

func checkOutcome(ctx context.Context, o Options, directory, commit string) error {
	return material.CheckOutcome(ctx, repository.Repository{}, o.Repository, o.Project, directory, commit, false)
}
