package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/service"
)

// Run dispatches an explicit CLI operation. All workflow writes go through its resident service.
func Run(ctx context.Context, o Options, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command required")
	}
	switch args[0] {
	case "config":
		if len(args) != 1 {
			return fmt.Errorf("config takes no arguments")
		}
		config, err := resolveProject(o)
		if err != nil {
			return err
		}
		return printJSON(config)
	case "candidate":
		if len(args) > 1 && args[1] == "material" {
			return candidateMaterial(ctx, o, args[2:])
		}
		return candidateDiff(ctx, args[1:])
	case "conversation":
		return exportConversation(args[1:])
	case "document":
		return provost(ctx, o, args[1:])
	case "role":
		return commissarSubagent(ctx, args[1:])
	case "assemble-role":
		if len(args) != 3 {
			return fmt.Errorf("assemble-role <role> <output-file>")
		}
		text, err := project.AssembleRole(o.Repository, o.Project, args[1])
		if err != nil {
			return err
		}
		return writeFile(args[2], []byte(text))
	case "check-assignment":
		if len(args) != 2 {
			return fmt.Errorf("check-assignment <assignment-file>")
		}
		return CheckAssignment(o, args[1])
	case "check-outcome-reports":
		if len(args) != 3 {
			return fmt.Errorf("check-outcome-reports <record-directory> <commit>")
		}
		return checkOutcome(ctx, o, args[1], args[2])
	case "plan-one-rule-gate":
		if len(args) != 2 {
			return fmt.Errorf("plan-one-rule-gate <instance-directory>")
		}
		return planOneRuleGate(o.Repository, args[1])
	case "batch":
		return batch(ctx, o, args[1:])
	case "slot":
		return slot(ctx, o, args[1:])
	case "review":
		return review(ctx, o, args[1:])
	case "events":
		if len(args) != 2 || args[1] != "watch" {
			return fmt.Errorf("events watch (select resident with --url)")
		}
		return o.watch(ctx, "", false)
	case "tick":
		flags := flag.NewFlagSet("tick", flag.ContinueOnError)
		n := flags.Int("ticks", 1, "number of explicit scenario steps")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || *n < 1 {
			return fmt.Errorf("tick requires --ticks positive integer")
		}
		var result any
		if err := o.request(ctx, "POST", "/api/tick", map[string]int{"ticks": *n}, &result); err != nil {
			return err
		}
		return printJSON(result)
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("status takes no arguments")
		}
		frame, err := o.frame(ctx, []string{"proposal/", "ledger", "slot/", "feed"}, "", 0, false)
		if err != nil {
			return err
		}
		return printJSON(frame)
	case "start", "run", "attach", "stop", "resume", "reset", "restart", "republish", "land", "end-early", "answer":
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
		c := service.Command{Action: strings.ReplaceAll(args[0], "-", "_")}
		flags.StringVar(&c.Orchestrator, "orchestrator", "", "orchestrator identity for commit trailers")
		flags.StringVar(&c.ID, "command-id", "", "idempotency ID; reuse exactly on retry")
		flags.StringVar(&c.UID, "uid", "", "proposal UID")
		flags.StringVar(&c.Slot, "slot", "", "slot label")
		flags.StringVar(&c.Directory, "directory", "", "slot worktree")
		flags.StringVar(&c.Folder, "folder", "", "repository-relative proposal folder")
		flags.StringVar(&c.Base, "base", "", "pinned base commit")
		flags.StringVar(&c.Rank, "rank", "", "mission rank")
		flags.StringVar(&c.Source, "source", "", "reason or source")
		flags.StringVar(&c.Kind, "kind", "", "end-early settlement: rejected, abandoned or retired")
		assignment := flags.String("assignment", "", "assignment file")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 || c.UID == "" {
			return fmt.Errorf("%s requires --uid and no positional arguments", args[0])
		}
		if args[0] == "end-early" {
			c.Action = "retire"
		}
		if args[0] == "attach" {
			return o.watch(ctx, c.UID, true)
		}
		if args[0] == "start" || args[0] == "run" {
			if c.Slot == "" || c.Rank == "" || *assignment == "" {
				return fmt.Errorf("start requires --slot, --rank and --assignment")
			}
			if err := CheckAssignment(o, *assignment); err != nil {
				return err
			}
			text, err := os.ReadFile(*assignment)
			if err != nil {
				return err
			}
			c.Assignment = string(text)
			c.Action = "start"
		}
		if err := o.submit(ctx, c); err != nil {
			return err
		}
		if args[0] == "run" {
			return o.watch(ctx, c.UID, true)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func candidateDiff(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "diff" {
		return fmt.Errorf("candidate diff <repository> <base> <candidate> [--name-only]")
	}
	names := false
	pos := []string{}
	for _, arg := range args[1:] {
		if arg == "--name-only" {
			names = true
		} else {
			pos = append(pos, arg)
		}
	}
	if len(pos) != 3 {
		return fmt.Errorf("candidate diff requires repository, base and candidate")
	}
	commits := []string{}
	for _, ref := range pos[1:] {
		out, err := exec.CommandContext(ctx, "git", "-C", pos[0], "rev-parse", "--verify", ref+"^{commit}").Output()
		if err != nil {
			return err
		}
		commits = append(commits, strings.TrimSpace(string(out)))
	}
	gitArgs := []string{"-C", pos[0], "diff", "--no-ext-diff", "--no-color"}
	if names {
		gitArgs = append(gitArgs, "--name-only", "--no-renames")
	} else {
		gitArgs = append(gitArgs, "--find-renames", "--unified=3")
	}
	gitArgs = append(gitArgs, commits[0], commits[1], "--")
	command := exec.CommandContext(ctx, "git", gitArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func batch(ctx context.Context, o Options, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("batch requires open|add|dispatch|check|settle|withdraw|approve|edit|drop|validate|close|transfer")
	}
	flags := flag.NewFlagSet("batch "+args[0], flag.ContinueOnError)
	a := ledger.Action{Kind: args[0]}
	var id string
	flags.StringVar(&id, "command-id", "", "idempotency ID")
	flags.StringVar(&a.Batch, "id", "", "batch ID")
	flags.StringVar(&a.Batch, "batch", "", "batch ID")
	flags.StringVar(&a.UID, "uid", "", "proposal UID")
	flags.StringVar(&a.Title, "title", "", "title")
	flags.StringVar(&a.Scope, "description", "", "scope or batch description")
	flags.StringVar(&a.Value, "kind", "", "settlement kind")
	flags.StringVar(&a.Source, "source", "", "approval or settlement source")
	flags.StringVar(&a.Branch, "branch", "", "dispatched proposal branch")
	flags.StringVar(&a.OriginBatch, "origin-batch", "", "original batch for transferred work")
	flags.StringVar(&a.StartedAt, "started", "", "batch start date")
	flags.StringVar(&a.ApprovedAt, "at", "", "approval timestamp")
	flags.StringVar(&a.ApprovalText, "text", "", "user approval text")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected batch arguments")
	}
	if a.Kind == "check" || a.Kind == "validate" {
		frame, err := o.frame(ctx, []string{"ledger"}, "", 0, false)
		if err != nil {
			return err
		}
		var state ledger.Ledger
		if err := json.Unmarshal(frame.Puts["ledger"], &state); err != nil {
			return err
		}
		if err := state.Validate(); err != nil {
			return err
		}
		if a.Kind == "check" {
			m := state.Members[a.UID]
			if m == nil || m.Settlement != "" || len(state.Stack) == 0 || m.Batch != state.Stack[0] {
				return fmt.Errorf("proposal must be an unsettled top-batch member")
			}
		}
		return printJSON(state)
	}
	return o.submit(ctx, service.Command{ID: id, Action: "ledger", Ledger: &a})
}

func resolveProject(o Options) (*project.Config, error) {
	if o.Repository == "" {
		return nil, fmt.Errorf("--repo required")
	}
	return project.Load(o.Repository, o.Main, o.Project)
}
func absolute(path string) string {
	value, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return value
}
