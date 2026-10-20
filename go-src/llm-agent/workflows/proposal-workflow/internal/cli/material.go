package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"proposal-platform/api"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/workflow"
)

// candidateMaterial prepares the same immutable surfaces as the workflow's delivery.
func candidateMaterial(ctx context.Context, o Options, args []string) error {
	if len(args) != 5 {
		return fmt.Errorf("candidate material <worktree> <commit> <folder> <outcome-directory-or-dash> <candidate-json>")
	}
	config, err := resolveProject(o)
	if err != nil {
		return err
	}
	repo := repository.Repository{Config: config}
	raw, err := os.ReadFile(args[4])
	if err != nil {
		return err
	}
	var candidate api.Candidate
	if err := material.Decode(string(raw), &candidate); err != nil {
		return err
	}
	manifestRaw, err := repo.Read(ctx, args[0], args[1], args[2]+"/revision.json")
	if err != nil {
		return err
	}
	var manifest material.Manifest
	if err := material.Decode(manifestRaw, &manifest); err != nil {
		return err
	}
	candidate.ID = args[1]
	candidate.Proposal = filepath.Base(args[2])
	candidate.Kind = manifest.Kind
	if candidate.Base == "" {
		candidate.Base = manifest.Base
	}
	task := workflow.Task{Proposal: candidate.Proposal, Directory: absolute(args[0]), Candidate: &candidate, Work: workflow.Work{Base: candidate.Base, Parent: manifest.Base, Folder: args[2]}}
	if args[3] != "-" {
		task.Work.Outcome = args[3]
		docs, err := material.ReadOutcomeDocuments(args[3], args[1])
		if err != nil {
			return err
		}
		candidate.Documents = append(candidate.Documents, docs...)
	}
	builder := material.Builder{Config: config, Repository: repo, Chow: o.Chow, Renderer: o.Renderer}
	candidate.Doubts, err = builder.Doubts(ctx, task, candidate.Doubts)
	if err != nil {
		return err
	}
	result, err := builder.Build(ctx, task, args[1])
	if err != nil {
		return err
	}
	return printJSON(result)
}
