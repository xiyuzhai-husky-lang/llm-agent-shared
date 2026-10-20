package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Runner executes external tools; it never changes workflow projections.
type Runner struct{}

func (Runner) Run(ctx context.Context, directory string, environment []string, input string, logPath string, name string, args ...string) (string, error) {
	executable, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("missing tool %s: %w", name, err)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = directory
	cmd.Env = append(os.Environ(), environment...)
	cmd.Stdin = strings.NewReader(input)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	out := &Output{Limit: 24 << 20}
	diagnostic := &Output{Limit: 1 << 20}
	var log *os.File
	if logPath != "" {
		if err = os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
			return "", err
		}
		log, err = os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", err
		}
		defer log.Close()
		fmt.Fprintf(log, "command: %q %q\ndirectory: %s\n", name, args, directory)
		cmd.Stdout = io.MultiWriter(out, log)
		cmd.Stderr = io.MultiWriter(diagnostic, log)
	} else {
		cmd.Stdout = out
		cmd.Stderr = diagnostic
	}
	err = cmd.Run()
	if log != nil {
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		} else if err != nil {
			code = -1
		}
		fmt.Fprintf(log, "\nexit_code: %d\n", code)
		if syncErr := log.Sync(); err == nil {
			err = syncErr
		}
	}
	if err != nil {
		message := diagnostic.String()
		if strings.TrimSpace(message) == "" {
			message = out.String()
			if len(message) > 4096 {
				message = message[len(message)-4096:]
			}
		}
		return out.String(), fmt.Errorf("%s: %w: %s", name, err, message)
	}
	return out.String(), nil
}
