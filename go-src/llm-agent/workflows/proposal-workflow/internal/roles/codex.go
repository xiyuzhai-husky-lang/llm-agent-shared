package roles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"proposal-workflow/internal/command"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/workflow"
)

// Codex invokes real roles, preserving their sessions and raw reports.
type Codex struct {
	Config   *project.Config
	Roles    *project.Roles
	Commands command.Runner
}

func (c Codex) Run(ctx context.Context, t workflow.Task, role, prompt string) (string, string, error) {
	directory := filepath.Join(c.Config.State, "runs", t.Proposal, t.ID)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", "", err
	}
	report := filepath.Join(directory, role+".md")
	gitDirectory, err := c.Commands.Run(ctx, t.Directory, nil, "", "", "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	roots, _ := json.Marshal(append([]string{strings.TrimSpace(gitDirectory)}, c.Config.CodexAddDirs...))
	sandbox := c.Config.RoleSandbox
	if sandbox == "none" {
		sandbox = "danger-full-access"
	}
	args := []string{"exec", "--ignore-user-config", "--json", "-m", t.Policy.Model, "-c", "model_reasoning_effort=" + strconv.Quote(t.Policy.Effort), "-c", "approval_policy=\"never\"", "-c", "sandbox_mode=" + strconv.Quote(sandbox), "-c", "project_doc_max_bytes=0", "-c", "sandbox_workspace_write.writable_roots=" + string(roots), "-c", "developer_instructions=" + strconv.Quote(c.Roles.Documents[role]+"\nThe Go workflow driver runs the physical gates, Outcome, Chow comparison and decision renderer. Return the role report; lifecycle commands belong to the driver.\n"), "-o", report}
	session := t.Work.Sessions[role]
	if session != "" {
		args = append(args, "resume", session, "-")
	} else {
		args = append(args, "-")
	}
	if err := os.WriteFile(filepath.Join(directory, role+"-prompt.txt"), []byte(prompt), 0600); err != nil {
		return "", session, err
	}
	output, err := c.Commands.Run(ctx, t.Directory, nil, prompt, filepath.Join(directory, role+".log"), "codex", args...)
	for _, line := range strings.Split(output, "\n") {
		var event struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Type == "thread.started" {
			session = event.ThreadID
		}
	}
	if err != nil {
		return "", session, err
	}
	if session == "" {
		return "", "", fmt.Errorf("codex did not report a thread ID")
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return "", session, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", session, fmt.Errorf("codex returned an empty report")
	}
	return string(data), session, nil
}

func Prompt(t workflow.Task, c *project.Config, role string) string {
	var text strings.Builder
	if t.Orchestrator != "" {
		fmt.Fprintf(&text, "Required commit trailer: Committed-By: subagent of orchestrator %s (%s)\n", t.Orchestrator, strings.ReplaceAll(c.ProjectKey, "/", "-"))
	}
	fmt.Fprintf(&text, "[RANK %s] model=%s effort=%s Worker revision cap=%d deadline=%ds (advisory)\n", t.Policy.Rank, t.Policy.Model, t.Policy.Effort, t.Policy.WorkerRevisions, t.Policy.DeadlineSeconds)
	fmt.Fprintf(&text, "You are the %s role. Assigned slot: %s\nBranch: %s%s\nMain checkout: %s\nProject: %s\nProposal folder: %s\nPinned base: %s\nPhase: %s; purpose: %s\n", role, t.Directory, c.BranchPrefix, t.Proposal, c.Main, c.Project, t.Work.Folder, t.Work.Base, t.Phase, t.Purpose)
	fmt.Fprintf(&text, "Read projects/common/review/README.md and %s/docs/proposal-authoring-guidance.md. The assembled developer instructions are your role document.\n\nAssignment verbatim:\n%s\n\nUser feedback verbatim:\n%s\n", c.Project, t.Assignment, t.UserText)
	if t.Diagnostic != "" {
		fmt.Fprintf(&text, "Previous operation report/diagnostics, verbatim:\n%s\n", t.Diagnostic)
	}
	if t.Candidate != nil {
		fmt.Fprintf(&text, "Current committed candidate: %s (%s)\n", t.Candidate.ID, t.Candidate.Kind)
	}
	if t.Work.Outcome != "" {
		fmt.Fprintf(&text, "Measured Outcome reports (read run.json and its native reports): %s\n", t.Work.Outcome)
	}
	if len(t.Work.MustSign) > 0 {
		fmt.Fprintln(&text, "MUST SIGN files computed by the work-step gate:")
		for _, path := range t.Work.MustSign {
			fmt.Fprintf(&text, "- %s %s\n", path, t.Work.MustSignReasons[path])
		}
	}
	if len(t.Feedback) > 0 {
		fmt.Fprintln(&text, "Findings to answer in order, preserving their words:")
		for i, d := range t.Feedback {
			fmt.Fprintf(&text, "%d. [%s] %s\n", i+1, d.ID, d.Text)
		}
	}
	if role == "commissar" {
		fmt.Fprintln(&text, "Read doctrines/README.md and select warranted doctrines for this candidate. Return Doctrines: and Convictions:; none when no conviction. Any Halt: must retain its reason.")
	}
	if t.Phase == "response" {
		fmt.Fprintln(&text, "RESPONSE-ONLY TURN. Change no files, commits or candidate bytes. Return Dispositions: with numbered agreed: or declined: answers.")
	}
	if t.Phase == "worker" {
		fmt.Fprintln(&text, "Return Step result: strict JSON on its own line, Dispositions: when answering findings, and Loop decision: review|present with Loop reason:. Do not author explanation surfaces yet.")
	}
	if t.Phase == "wrapup" {
		fmt.Fprintf(&text, "Wrap up only %s. Settled implementation: %s. Step question: %s\nChow problems: %q\nUse revision.json format 4, base %s and kind %s. Commit the completed folder. Do not change implementation or rerun Outcome.\n", t.Work.Folder, t.Candidate.ID, t.Work.Question, t.Work.Problems, t.Work.Parent, t.Candidate.Kind)
	}
	return text.String()
}
