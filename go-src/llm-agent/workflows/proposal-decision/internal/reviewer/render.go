package reviewer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"proposal-decision/internal/candidate"
	"proposal-platform/api"
)

// RenderFile projects two explicit texts through the decision page's own diff renderer.
func RenderFile(ctx context.Context, path, before, after string) (any, error) {
	directory, err := os.MkdirTemp("", "proposal-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	oldPath, newPath := filepath.Join(directory, "before"), filepath.Join(directory, "after")
	if err := os.WriteFile(oldPath, []byte(before), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(newPath, []byte(after), 0600); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-color", "--unified=3", oldPath, newPath)
	patch, err := command.Output()
	if exit, ok := err.(*exec.ExitError); err != nil && (!ok || exit.ExitCode() != 1) {
		return nil, err
	}
	hunks, err := candidate.ParsePatch(string(patch), nil)
	if err != nil {
		return nil, err
	}
	for i := range hunks {
		hunks[i].Path = path
	}
	file := File{Path: path, Hunks: hunks}
	material := Candidate{Material: api.Candidate{Base: "before", ID: "after"}, Sources: map[string]string{"before:" + path: before, "after:" + path: after}}
	if err := file.prepare(ctx, &material); err != nil {
		return nil, err
	}
	return map[string]any{"blocks": file.Blocks, "notice": file.Notice, "path": path}, nil
}

// ReadRenderRequest deliberately accepts text, never arbitrary filesystem paths to read.
func RenderDescription(path string) error {
	if path == "" || strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("render path required")
	}
	return nil
}
