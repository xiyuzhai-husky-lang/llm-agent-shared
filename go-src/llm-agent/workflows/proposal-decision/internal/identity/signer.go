package identity

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Signer is the served repository's identity, resolved once at startup.
type Signer struct{ Name string }

func Read(ctx context.Context, repository string) (Signer, error) {
	output, err := exec.CommandContext(ctx, "git", "-C", repository, "config", "user.name").CombinedOutput()
	if err != nil {
		return Signer{}, fmt.Errorf("git config user.name in %s: %w: %s", repository, err, strings.TrimSpace(string(output)))
	}
	name := strings.TrimSpace(string(output))
	if name == "" {
		return Signer{}, fmt.Errorf("git config user.name in %s is empty", repository)
	}
	return Signer{Name: name}, nil
}
