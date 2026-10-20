package reviewer

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// readGit always reads the explicitly selected repository, regardless of the
// caller's Git environment. Reads neither refresh the index nor fetch objects.
func readGit(ctx context.Context, repository string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repository}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1")
	return command
}
