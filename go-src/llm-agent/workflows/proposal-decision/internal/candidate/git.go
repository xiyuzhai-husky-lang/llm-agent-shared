package candidate

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"proposal-platform/api"
	"regexp"
)

// Git owns the read-only Git diff invocation used by candidate construction.
type Git struct{}

func (Git) Patch(ctx context.Context, source api.Source) (string, error) {
	commit := regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	if !commit.MatchString(source.Base) || !commit.MatchString(source.Commit) {
		return "", fmt.Errorf("Git source requires full pinned commit IDs")
	}
	command := exec.CommandContext(ctx, "git", "-C", source.Repository, "-c", "core.quotePath=true", "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--binary", "--unified=3", source.Base, source.Commit, "--")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err = command.Start(); err != nil {
		return "", err
	}
	output, readErr := io.ReadAll(io.LimitReader(stdout, (24<<20)+1))
	if len(output) > 24<<20 {
		_ = command.Process.Kill()
		_ = command.Wait()
		return "", fmt.Errorf("candidate diff exceeds 24 MiB")
	}
	waitErr := command.Wait()
	if readErr != nil {
		return "", readErr
	}
	if waitErr != nil {
		return "", fmt.Errorf("read candidate diff: %w", waitErr)
	}
	return string(output), nil
}
