package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"proposal-workflow/internal/workflow"
)

// Process owns per-phase argv; each command consumes a Task and emits a Result.
type Process struct {
	Commands map[string][]string `json:"commands"`
}

func LoadProcess(path string) (*Process, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var process Process
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&process); err != nil {
		return nil, err
	}
	return &process, nil
}

func (p *Process) Run(ctx context.Context, task workflow.Task) (workflow.Result, error) {
	argv := p.Commands[task.Phase]
	if len(argv) == 0 {
		return workflow.Result{}, fmt.Errorf("no executable configured for %s", task.Phase)
	}
	input, err := json.Marshal(task)
	if err != nil {
		return workflow.Result{}, err
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = task.Directory
	command.Stdin = bytes.NewReader(input)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 2 * time.Second
	output := &boundedBuffer{limit: 24 << 20}
	diagnostic := &boundedBuffer{limit: 1 << 20}
	command.Stdout = output
	command.Stderr = diagnostic
	if err = command.Run(); err != nil {
		return workflow.Result{}, fmt.Errorf("%s failed: %w: %s", task.Phase, err, diagnostic.buffer.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(output.buffer.Bytes()))
	decoder.DisallowUnknownFields()
	var result workflow.Result
	if err = decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("%s result: %w", task.Phase, err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return result, fmt.Errorf("expected one task result")
	}
	return result, nil
}
