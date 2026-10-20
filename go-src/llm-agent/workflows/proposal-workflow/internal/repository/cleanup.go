package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (repo Repository) CheckRelease(ctx context.Context, path, target string) error {
	if _, err := repo.Git(ctx, path, "cat-file", "-e", target+"^{commit}"); err != nil {
		return err
	}
	for _, name := range []string{"index.lock", "HEAD.lock", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
		location, err := repo.Git(ctx, path, "rev-parse", "--path-format=absolute", "--git-path", name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(location); err == nil {
			return fmt.Errorf("Git operation or lock exists: %s", location)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	tree, err := repo.Git(ctx, path, "ls-tree", "-r", "--name-only", "-z", target)
	if err != nil {
		return err
	}
	ignored, err := repo.Git(ctx, path, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	for _, ignore := range strings.Split(ignored, "\x00") {
		if ignore == "" {
			continue
		}
		for _, tracked := range strings.Split(tree, "\x00") {
			if tracked != "" && (tracked == ignore || strings.HasPrefix(tracked, ignore+"/") || strings.HasPrefix(ignore, tracked+"/")) {
				return fmt.Errorf("target would overwrite ignored path %s", ignore)
			}
		}
	}
	untracked, err := repo.Git(ctx, path, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(untracked, "\x00") {
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, ".local/") {
			return fmt.Errorf(".local evidence must be ignored: %s", entry)
		}
		if _, err := os.Stat(filepath.Join(path, entry, ".git")); err == nil {
			return fmt.Errorf("untracked nested repository: %s", entry)
		}
	}
	return nil
}

// Release discards tracked and nonignored work only after checking the exact target.
func (repo Repository) Release(ctx context.Context, path, target string) error {
	if err := repo.CheckRelease(ctx, path, target); err != nil {
		return err
	}
	for _, args := range [][]string{{"restore", "--source=HEAD", "--staged", "--worktree", "--", "."}, {"clean", "-nd"}, {"clean", "-fd"}, {"switch", "--detach", "--no-overwrite-ignore", target}} {
		if _, err := repo.Git(ctx, path, args...); err != nil {
			return fmt.Errorf("slot release stopped at git %s: %w", strings.Join(args, " "), err)
		}
		fmt.Println("completed git " + strings.Join(args, " "))
	}
	if err := repo.Clean(ctx, path); err != nil {
		return err
	}
	actual, err := repo.Git(ctx, path, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if actual != target {
		return fmt.Errorf("released HEAD differs from origin target")
	}
	return nil
}
