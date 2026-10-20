package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"proposal-platform/api"
	"proposal-workflow/internal/gates"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/workflow"
)

func review(ctx context.Context, o Options, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("review requires serve|check-surfaces")
	}
	switch args[0] {
	case "serve":
		flags := flag.NewFlagSet("review serve", flag.ContinueOnError)
		listen := flags.String("listen", "127.0.0.1:8801", "decision server address")
		web := flags.String("web-root", filepath.Join(o.Repository, o.Project, "proposal-ui/dist"), "built decision frontend")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected review serve arguments")
		}
		command := exec.CommandContext(ctx, filepath.Join(o.Repository, "scripts/proposal-tool.sh"), "decision", "--repository", o.Main, "--listen", *listen, "--web", *web, "--workflow-url", o.URL, "--peek-script", filepath.Join(o.Repository, "webui/apps/proposal-decision/server/peek.ts"))
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		return command.Run()
	case "check-surfaces":
		if len(args) != 5 {
			return fmt.Errorf("review check-surfaces <worktree> <candidate> <folder> <main>")
		}
		o.Main = args[4]
		config, err := resolveProject(o)
		if err != nil {
			return err
		}
		repo := repository.Repository{Config: config}
		commit, err := repo.Git(ctx, args[1], "rev-parse", "--verify", args[2]+"^{commit}")
		if err != nil {
			return err
		}
		raw, err := repo.Read(ctx, args[1], commit, args[3]+"/revision.json")
		if err != nil {
			return err
		}
		var manifest material.Manifest
		if err := material.Decode(raw, &manifest); err != nil {
			return err
		}
		uid := filepath.Base(args[3])
		task := workflow.Task{ID: "check-surfaces", Proposal: uid, Directory: absolute(args[1]), Phase: "surfaces", Work: workflow.Work{Folder: args[3], Parent: manifest.Base}, Candidate: &api.Candidate{ID: commit, Kind: manifest.Kind}}
		task.Work.Base, err = repo.Git(ctx, args[1], "merge-base", config.TargetBranch, commit)
		if err != nil {
			return err
		}
		if manifest.Kind == "complete" {
			frame, err := o.frame(ctx, []string{"proposal/" + uid}, "", 0, false)
			if err != nil {
				return err
			}
			var proposal workflow.Proposal
			if err := json.Unmarshal(frame.Puts["proposal/"+uid], &proposal); err != nil {
				return err
			}
			if proposal.Work.Outcome == "" {
				return fmt.Errorf("proposal %s has no measured Outcome", uid)
			}
			task.Work.Outcome = proposal.Work.Outcome
			documents, err := material.OutcomeDocuments(task.Work.Outcome, proposal.Work.Reviewed)
			if err != nil {
				return err
			}
			task.Candidate.Documents = documents
		}
		builder := material.Builder{Config: config, Repository: repo, Chow: o.Chow, Renderer: o.Renderer}
		_, err = (gates.Gates{Config: config, Repository: repo, Materials: builder}).Run(ctx, task)
		return err
	default:
		return fmt.Errorf("unknown review command %q; deliveries update the Go reviewer directly", args[0])
	}
}
