package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"proposal-workflow/internal/command"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/workflow"
)

// Repository owns Git effects in the assigned slot and pinned reads.
type Repository struct {
	Commands command.Runner
	Config   *project.Config
}

func (r Repository) Git(ctx context.Context, directory string, args ...string) (string, error) {
	output, err := r.Commands.Run(ctx, directory, nil, "", "", "git", args...)
	return strings.TrimSpace(output), err
}
func (r Repository) Clean(ctx context.Context, directory string) error {
	status, err := r.Git(ctx, directory, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("dirty slot %s:\n%s", directory, status)
	}
	return nil
}
func (r Repository) Prepare(ctx context.Context, t workflow.Task) (string, error) {
	branch := r.Config.BranchPrefix + t.Proposal
	if _, err := os.Stat(t.Directory); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(t.Directory), 0700); err != nil {
			return "", err
		}
		if _, err = r.Git(ctx, r.Config.Main, "worktree", "add", "-b", branch, t.Directory, r.Config.TargetBranch); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	root, err := r.Git(ctx, t.Directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if root != t.Directory {
		return "", fmt.Errorf("slot must name the worktree root %s", root)
	}
	actual, err := r.Git(ctx, t.Directory, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if err = r.Clean(ctx, t.Directory); err != nil {
		return "", err
	}
	common, err := r.Git(ctx, t.Directory, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	mainCommon, err := r.Git(ctx, r.Config.Main, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if common != mainCommon {
		return "", fmt.Errorf("slot belongs to a different repository")
	}
	if actual != branch {
		owned := filepath.Join(r.Config.State, "slots", t.Slot)
		if t.Directory != owned || t.Purpose != "initial" {
			return "", fmt.Errorf("slot branch %s; expected %s", actual, branch)
		}
		if _, err = r.Git(ctx, t.Directory, "switch", "-c", branch, r.Config.TargetBranch); err != nil {
			return "", err
		}
	}
	base, err := r.Git(ctx, t.Directory, "merge-base", r.Config.TargetBranch, "HEAD")
	if err != nil {
		return "", err
	}
	if t.Work.Base != "" && t.Work.Base != base {
		return "", fmt.Errorf("pinned base %s differs from slot branch point %s", t.Work.Base, base)
	}
	if err = r.prepareSkills(t.Directory); err != nil {
		return "", err
	}
	return base, nil
}
func (r Repository) Read(ctx context.Context, directory, oid, path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("invalid repository path %s", path)
	}
	return r.Commands.Run(ctx, directory, nil, "", "", "git", "show", oid+":"+filepath.ToSlash(path))
}
func (r Repository) Changes(ctx context.Context, directory, base, head string) ([]string, error) {
	out, err := r.Commands.Run(ctx, directory, nil, "", "", "git", "diff", "--name-only", "-z", "--no-renames", base, head, "--")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00"), nil
}
