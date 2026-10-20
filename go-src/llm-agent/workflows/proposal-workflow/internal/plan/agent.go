package plan

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
)

type Execution struct {
	Session string `json:"session"`
	Message string `json:"message"`
}

func (s *Service) executeRole(ctx context.Context, state *State, role string, correction int, prompt string) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	input := clone(state)
	type result struct {
		message string
		err     error
	}
	done := make(chan result, 1)
	go func() {
		message, err := s.executeRoleProcess(ctx, input, role, correction, prompt)
		done <- result{message, err}
	}()
	var deadline <-chan time.Time
	if !state.DeadlineRecorded {
		timer := time.NewTimer(time.Until(time.Unix(state.StartedAt, 0).Add(time.Duration(state.Rank.DeadlineMinutes) * time.Minute)))
		defer timer.Stop()
		deadline = timer.C
	}
	for {
		select {
		case value := <-done:
			state.Sessions = input.Sessions
			return value.message, value.err
		case <-deadline:
			deadline = nil
			state.DeadlineRecorded = true
			state.record("advisory deadline passed; run continues")
			if err := s.save(state); err != nil {
				cancel()
				<-done
				return "", err
			}
		}
	}
}

func (s *Service) executeRoleProcess(ctx context.Context, state *State, role string, correction int, prompt string) (string, error) {
	directory := s.runDirectory(state)
	childPath := filepath.Join(directory, "child.json")
	if err := reclaimChild(ctx, childPath); err != nil {
		return "", err
	}
	label := fmt.Sprintf("%s-round-%d-correction-%d", role, state.Round, correction)
	reportPath := filepath.Join(directory, label+".md")
	receiptPath := filepath.Join(directory, label+".execution.json")
	sessionPath := filepath.Join(directory, label+".session")
	if raw, err := os.ReadFile(receiptPath); err == nil {
		var receipt Execution
		if err := material.Decode(string(raw), &receipt); err != nil {
			return "", err
		}
		if receipt.Session == "" {
			return "", fmt.Errorf("empty execution session")
		}
		state.Sessions[role] = receipt.Session
		return receipt.Message, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	session := state.Sessions[role]
	if raw, err := os.ReadFile(sessionPath); err == nil {
		if session != "" && session != string(raw) {
			return "", fmt.Errorf("turn session mismatch")
		}
		session = string(raw)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	logPath := filepath.Join(directory, label+".log")
	if _, err := os.Stat(logPath); err == nil && session == "" {
		return "", fmt.Errorf("interrupted launch has no recorded session: %s", label)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := s.CheckAssignment(filepath.Join(directory, "assignment.md")); err != nil {
		return "", err
	}
	charter, err := project.AssembleRole(s.Config.Repository, s.Config.Project, role)
	if err != nil {
		return "", err
	}
	prompt = charter + "\n\n" + prompt
	if err := os.WriteFile(filepath.Join(directory, label+".prompt.md"), []byte(prompt), 0600); err != nil {
		return "", err
	}
	readOnly := role == "plan-reviewer" || correction > 0 || state.current().ResponseOnly
	binary := "codex"
	args := []string{}
	if strings.HasPrefix(state.Rank.Model, "claude-") {
		if state.Rank.Effort == "ultra" {
			return "", fmt.Errorf("Claude Code has no reasoning effort ultra")
		}
		binary = "claude"
		args = []string{"-p", "--model", state.Rank.Model, "--effort", state.Rank.Effort, "--output-format", "stream-json", "--verbose", "--permission-mode", "bypassPermissions", "--strict-mcp-config"}
		if readOnly {
			args = append(args, "--disallowedTools", "Edit", "Write", "NotebookEdit")
		}
		for _, path := range s.Config.CodexAddDirs {
			args = append(args, "--add-dir", path)
		}
		if session != "" {
			args = append(args, "--resume", session)
		}
	} else if strings.HasPrefix(state.Rank.Model, "gpt-") {
		sandbox := "workspace-write"
		if readOnly {
			sandbox = "read-only"
		}
		args = []string{"exec", "--ignore-user-config", "-C", s.Config.Repository, "-m", state.Rank.Model, "-c", "model_reasoning_effort=" + strconv.Quote(state.Rank.Effort), "-c", "approval_policy=\"never\"", "-c", "project_doc_max_bytes=0", "-s", sandbox}
		for _, path := range s.Config.CodexAddDirs {
			args = append(args, "--add-dir", path)
		}
		if session != "" {
			args = append(args, "resume")
		}
		args = append(args, "--json", "-o", reportPath)
		if session != "" {
			args = append(args, session)
		}
		args = append(args, "-")
	} else {
		return "", fmt.Errorf("unsupported agent model %s", state.Rank.Model)
	}
	stderr, err := os.OpenFile(filepath.Join(directory, label+".stderr.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer stderr.Close()
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer log.Close()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = s.Config.Repository
	command.Stdin = strings.NewReader(prompt)
	command.Stderr = stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = 5 * time.Second
	command.Env = []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "CLAUDECODE=") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "GIT_OPTIONAL_LOCKS=0")
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := command.Start(); err != nil {
		return "", err
	}
	started, err := processStart(ctx, command.Process.Pid)
	if err != nil || started == "" {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
		return "", fmt.Errorf("cannot bind started role process: %v", err)
	}
	childRaw, _ := json.Marshal(childProcess{command.Process.Pid, started})
	if err := writeAtomic(childPath, childRaw); err != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
		return "", err
	}
	defer os.Remove(childPath)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	observed := ""
	claudeMessage := ""
	var streamErr error
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := log.Write(append(append([]byte{}, line...), '\n')); err != nil {
			streamErr = err
			break
		}
		var event struct {
			Type      string  `json:"type"`
			Subtype   string  `json:"subtype"`
			ThreadID  string  `json:"thread_id"`
			SessionID string  `json:"session_id"`
			IsError   *bool   `json:"is_error"`
			Result    *string `json:"result"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			streamErr = fmt.Errorf("malformed agent JSON: %w", err)
			break
		}
		identity := ""
		if binary == "codex" && event.Type == "thread.started" {
			identity = event.ThreadID
			if identity == "" {
				streamErr = fmt.Errorf("thread.started lacks thread_id")
				break
			}
		}
		if binary == "claude" && event.Type == "system" && event.Subtype == "init" {
			identity = event.SessionID
			if identity == "" {
				streamErr = fmt.Errorf("system init lacks session_id")
				break
			}
		}
		if identity != "" {
			if (session != "" && session != identity) || (observed != "" && observed != identity) {
				streamErr = fmt.Errorf("unexpected agent session")
				break
			}
			observed = identity
			if err := writeAtomic(sessionPath, []byte(identity)); err != nil {
				streamErr = err
				break
			}
		}
		if binary == "claude" && event.Type == "result" {
			if event.IsError == nil || *event.IsError || event.Result == nil {
				streamErr = fmt.Errorf("Claude returned an error or invalid result")
				break
			}
			claudeMessage = *event.Result
		}
	}
	if streamErr == nil {
		streamErr = scanner.Err()
	}
	if streamErr != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	waitErr := command.Wait()
	if streamErr != nil {
		return "", streamErr
	}
	if waitErr != nil {
		return "", waitErr
	}
	if observed == "" {
		return "", fmt.Errorf("agent reported no session")
	}
	if binary == "claude" {
		if claudeMessage == "" {
			return "", fmt.Errorf("Claude reported no final message")
		}
		if err := os.WriteFile(reportPath, []byte(claudeMessage), 0600); err != nil {
			return "", err
		}
	}
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(raw)) == "" {
		return "", fmt.Errorf("empty role message")
	}
	receipt := Execution{observed, string(raw)}
	data, err := json.Marshal(receipt)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(receiptPath, data); err != nil {
		return "", err
	}
	state.Sessions[role] = observed
	state.record(label + " execution completed")
	if err := s.save(state); err != nil {
		return "", err
	}
	return string(raw), nil
}
