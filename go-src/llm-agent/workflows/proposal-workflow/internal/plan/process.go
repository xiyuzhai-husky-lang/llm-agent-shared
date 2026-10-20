package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type childProcess struct {
	PID     int    `json:"pid"`
	Started string `json:"started"`
}

func processStart(ctx context.Context, pid int) (string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-p", fmt.Sprint(pid), "-o", "lstart=").Output()
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(out)), err
}
func reclaimChild(ctx context.Context, path string) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var child childProcess
	if err := json.Unmarshal(raw, &child); err != nil {
		return err
	}
	if child.PID <= 1 || child.Started == "" {
		return fmt.Errorf("invalid owned child record")
	}
	actual, err := processStart(ctx, child.PID)
	if err != nil {
		return err
	}
	if actual != "" && actual != child.Started {
		return fmt.Errorf("owned child PID was reused; inspect %s", path)
	}
	if err := syscall.Kill(-child.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return os.Remove(path)
}
