package execution

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"proposal-platform/api"
	"proposal-workflow/internal/command"
	"proposal-workflow/internal/gates"
	"proposal-workflow/internal/landing"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/roles"
	"proposal-workflow/internal/workflow"
)

// Project composes real adapters; the service owns every phase transition.
type Project struct {
	Config     *project.Config
	Landing    *landing.Lander
	Repository repository.Repository
	Roles      roles.Codex
	Materials  material.Builder
	Gates      gates.Gates
	Commands   command.Runner
}

func NewProject(config *project.Config, chow, renderer string) (*Project, error) {
	for _, name := range []string{"git", "codex", "bun", "make", "royal-guard", "bash", "python3", chow} {
		if _, err := exec.LookPath(name); err != nil {
			return nil, fmt.Errorf("missing tool %s: %w", name, err)
		}
	}
	documents, err := project.ReadRoles(config)
	if err != nil {
		return nil, err
	}
	p := &Project{Config: config, Repository: repository.Repository{Config: config}}
	p.Roles = roles.Codex{Config: config, Roles: documents}
	p.Materials = material.Builder{Config: config, Repository: p.Repository, Chow: chow, Renderer: renderer}
	p.Gates = gates.Gates{Config: config, Repository: p.Repository, Materials: p.Materials}
	p.Landing = &landing.Lander{Config: config, Repository: p.Repository}
	return p, nil
}

func (p *Project) Run(ctx context.Context, t workflow.Task) (workflow.Result, error) {
	r, err := p.run(ctx, t)
	if err != nil {
		r.OK = false
		r.Detail = err.Error()
	}
	return r, nil
}
func (p *Project) run(ctx context.Context, t workflow.Task) (workflow.Result, error) {
	switch t.Phase {
	case "preflight":
		base, err := p.Repository.Prepare(ctx, t)
		if err != nil {
			return workflow.Result{}, err
		}
		t.Work.Base = base
		if t.Work.Parent == "" {
			t.Work.Parent = base
		}
		if !filepath.IsLocal(t.Work.Folder) || filepath.Base(t.Work.Folder) != t.Proposal {
			return workflow.Result{}, fmt.Errorf("folder must be a repository-relative proposal folder ending with UID")
		}
		return workflow.Result{OK: true, Work: &t.Work, Detail: "slot ready; base " + base}, nil
	case "worker", "response", "wrapup", "commissar", "reviewer", "nitpicker":
		return p.role(ctx, t)
	case "outcome":
		return p.outcome(ctx, t)
	case "retire-check", "retire-clean":
		return p.retire(ctx, t)
	case "land", "land-outcome":
		return p.Landing.Run(ctx, t)
	default:
		return p.Gates.Run(ctx, t)
	}
}
func (p *Project) outcome(ctx context.Context, t workflow.Task) (workflow.Result, error) {
	directory := filepath.Join(p.Config.State, "outcome", t.Proposal, t.ID)
	env := []string{"OUTCOME_BASE=" + t.Work.Base, "OUTCOME_LOCK=" + filepath.Join(p.Config.State, "outcome.lock")}
	_, runErr := p.Commands.Run(ctx, t.Directory, env, "", filepath.Join(p.Config.State, "runs", t.Proposal, t.ID, "outcome.log"), "bash", filepath.Join(t.Directory, "projects/common/outcome/run.sh"), filepath.Join(t.Directory, p.Config.Project), directory)
	validationErr := material.CheckOutcome(ctx, p.Repository, t.Directory, p.Config.Project, directory, t.Candidate.ID, true)
	documents, readErr := material.OutcomeDocuments(directory, t.Candidate.ID)
	if readErr == nil {
		readErr = validationErr
	}
	result := workflow.Result{OK: runErr == nil && readErr == nil, Documents: documents, Work: &t.Work}
	if runErr != nil {
		return result, runErr
	}
	if readErr != nil {
		return result, readErr
	}
	if err := p.Repository.Clean(ctx, t.Directory); err != nil {
		return result, err
	}
	result.Work.Outcome = directory
	result.Detail = "Outcome passed at " + t.Candidate.ID
	return result, nil
}

func reportDocument(t workflow.Task, role, text string) api.Document {
	return api.Document{ID: t.ID + "-report", Kind: "role", Title: role + " · " + t.ID, Text: text}
}
