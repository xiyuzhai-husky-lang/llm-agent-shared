package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"proposal-workflow/internal/command"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/workflow"
)

// Gates checks physical candidate bytes; role assertions cannot substitute for these checks.
type Gates struct {
	Config     *project.Config
	Repository repository.Repository
	Commands   command.Runner
	Materials  material.Builder
}

func (g Gates) Run(ctx context.Context, t workflow.Task) (workflow.Result, error) {
	result := workflow.Result{OK: true}
	head, err := g.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	if t.Candidate == nil || head != t.Candidate.ID {
		return result, fmt.Errorf("slot HEAD differs from the candidate")
	}
	if err = g.Repository.Clean(ctx, t.Directory); err != nil {
		return result, err
	}
	log := filepath.Join(g.Config.State, "runs", t.Proposal, t.ID, t.Phase+".log")
	switch t.Phase {
	case "worker-step":
		changes, err := g.Repository.Changes(ctx, t.Directory, t.Work.Parent, head)
		if err != nil {
			return result, err
		}
		for _, path := range changes {
			if strings.HasPrefix(path, t.Work.Folder+"/") && path != t.Work.Folder+"/must-sign-additions.json" {
				return result, fmt.Errorf("work step wrote explanation surface %s before wrap-up", path)
			}
		}
		work := t.Work
		work.MustSign, work.MustSignReasons, err = g.Materials.MustSign(ctx, t, head)
		if err != nil {
			return result, err
		}
		result.Work = &work
		_, err = g.Commands.Run(ctx, t.Directory, nil, "", log, "royal-guard", "--event", "GIT_COMMIT", "--base", t.Work.Base, "--head", head)
	case "project":
		if g.Config.ProjectGate != "" {
			_, err = g.Commands.Run(ctx, t.Directory, nil, "", log, filepath.Join(g.Config.Repository, g.Config.Project, g.Config.ProjectGate), t.Directory, t.Work.Base, head)
		}
	case "wrap-gate":
		changes, err := g.Repository.Changes(ctx, t.Directory, t.Work.Reviewed, head)
		if err != nil {
			return result, err
		}
		for _, path := range changes {
			if path == t.Work.Folder+"/must-sign-additions.json" {
				return result, fmt.Errorf("wrap-up changed settled MUST SIGN additions")
			}
			if !strings.HasPrefix(path, t.Work.Folder+"/") {
				return result, fmt.Errorf("wrap-up changed settled implementation: %s", path)
			}
		}
	case "surfaces":
		listing, err := g.Repository.Git(ctx, t.Directory, "ls-tree", "-r", "--name-only", head, "--", t.Work.Folder)
		if err != nil {
			return result, err
		}
		paths := map[string]bool{}
		for _, path := range strings.Split(listing, "\n") {
			paths[path] = true
		}
		for path := range paths {
			if strings.HasSuffix(path, ".kathy.json") && !paths[strings.TrimSuffix(path, ".kathy.json")+".chw.tex"] {
				return result, fmt.Errorf("Kathy surface has no committed Chow source: %s", path)
			}
		}
		for _, path := range strings.Split(listing, "\n") {
			if !strings.HasSuffix(path, ".chw.tex") {
				continue
			}
			source, err := g.Repository.Read(ctx, t.Directory, head, path)
			if err != nil {
				return result, err
			}
			converted, err := g.Commands.Run(ctx, t.Directory, nil, source, "", g.Materials.Chow, "-")
			if err != nil {
				return result, err
			}
			committed, err := g.Repository.Read(ctx, t.Directory, head, strings.TrimSuffix(path, ".chw.tex")+".kathy.json")
			if err != nil {
				return result, err
			}
			var a, b any
			if err = json.Unmarshal([]byte(converted), &a); err != nil {
				return result, err
			}
			if err = json.Unmarshal([]byte(committed), &b); err != nil {
				return result, err
			}
			if !reflect.DeepEqual(a, b) {
				return result, fmt.Errorf("committed Kathy differs from Chow conversion: %s", path)
			}
		}
		built, err := g.Materials.Build(ctx, t, head)
		if err != nil {
			return result, err
		}
		result.Candidate = built
	default:
		return result, fmt.Errorf("unknown gate %s", t.Phase)
	}
	if err != nil {
		return result, err
	}
	if err = g.Repository.Clean(ctx, t.Directory); err != nil {
		return result, err
	}
	result.Detail = t.Phase + " passed at " + head
	return result, nil
}
