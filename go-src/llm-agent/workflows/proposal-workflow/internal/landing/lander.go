package landing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"proposal-platform/api"
	"reflect"
	"strings"
	"sync"

	"proposal-workflow/internal/command"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/workflow"
)

// Lander serializes accepted-candidate landings and records each external boundary.
type Lander struct {
	Config     *project.Config
	Repository repository.Repository
	Commands   command.Runner
	mu         sync.Mutex
}

func (l *Lander) Run(ctx context.Context, t workflow.Task) (result workflow.Result, runErr error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	result = workflow.Result{}
	if t.Verdict == nil || t.Verdict.Value != "accept" || t.Candidate == nil || t.Verdict.Candidate != t.Candidate.ID {
		return result, fmt.Errorf("landing requires the standing Accept of this candidate")
	}
	c := l.Config
	if (c.LandingOutcome != "measure-landed-tree" && c.LandingOutcome != "validate-candidate-outcome" && c.LandingOutcome != "none") || (c.LandingMainChanges != "merge-check" && c.LandingMainChanges != "require-clean") || (c.LandingUIDCheck != "environment" && c.LandingUIDCheck != "main-argument") {
		return result, fmt.Errorf("unsupported project landing policy")
	}
	trailers := []string{}
	for _, binding := range c.LandingTrailerEnvironment {
		pair := strings.SplitN(binding, "=", 2)
		if len(pair) != 2 || os.Getenv(pair[1]) == "" {
			return result, fmt.Errorf("landing requires trailer environment %s", binding)
		}
		trailers = append(trailers, pair[0]+": "+os.Getenv(pair[1]))
	}
	trailers = append(trailers, c.LandingTrailerLiterals...)
	dir := filepath.Join(c.State, "landings", t.Proposal)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return result, err
	}
	path := filepath.Join(dir, "landing.json")
	record := Record{Candidate: t.Candidate.ID, Step: "prepare"}
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = material.Decode(string(raw), &record); err != nil {
			return result, err
		}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	if record.Candidate != t.Candidate.ID {
		return result, fmt.Errorf("a different candidate already owns this landing")
	}
	defer func() {
		if runErr != nil {
			runErr = fmt.Errorf("landing stopped at %s; review=%s merge=%s; record %s: %w", record.Step, record.Review, record.Merge, path, runErr)
		}
	}()
	save := func() error {
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		file, err := os.OpenFile(path+".next", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		return os.Rename(path+".next", path)
	}
	git := func(args ...string) (string, error) {
		if c.LandingMergeHooks == "skip" {
			args = append([]string{"-c", "core.hooksPath=/dev/null"}, args...)
		}
		output, err := l.Commands.Run(ctx, c.Main, c.LandingEnvironment, "", "", "git", args...)
		return strings.TrimSpace(output), err
	}
	branch, err := git("symbolic-ref", "--short", "HEAD")
	if err != nil {
		return result, err
	}
	if branch != c.TargetBranch {
		return result, fmt.Errorf("main branch %s; expected %s", branch, c.TargetBranch)
	}
	if record.Step == "prepare" {
		if c.LandingMainChanges == "require-clean" {
			changes, err := git("status", "--porcelain=v1", "--untracked-files=no")
			if err != nil {
				return result, err
			}
			if changes != "" {
				return result, fmt.Errorf("main has tracked changes:\n%s", changes)
			}
		}
		slotHead, err := l.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
		if err != nil {
			return result, err
		}
		if slotHead != record.Candidate {
			return result, fmt.Errorf("slot no longer points to the accepted candidate")
		}
		if err = l.Repository.Clean(ctx, t.Directory); err != nil {
			return result, err
		}
		if _, err = git("diff", "--cached", "--exit-code"); err != nil {
			return result, fmt.Errorf("main index must be clean before landing: %w", err)
		}
		for _, dependency := range c.LandingDependencies {
			info, err := os.Stat(filepath.Join(c.Main, c.Project, dependency))
			if err != nil {
				return result, err
			}
			if info.Mode()&0111 == 0 {
				return result, fmt.Errorf("landing dependency is not executable: %s", dependency)
			}
		}
		if c.ProjectGate != "" {
			base, err := l.Repository.Git(ctx, t.Directory, "merge-base", c.TargetBranch, t.Candidate.ID)
			if err != nil {
				return result, err
			}
			if _, err = l.Commands.Run(ctx, t.Directory, c.LandingEnvironment, "", filepath.Join(dir, t.ID+"-project.log"), filepath.Join(c.Repository, c.Project, c.ProjectGate), t.Directory, base, t.Candidate.ID); err != nil {
				return result, err
			}
		}
		uidArgs := []string{filepath.Join(c.Repository, c.Project, "tools/check-uids.sh")}
		if c.LandingUIDCheck == "main-argument" {
			uidArgs = append(uidArgs, c.Main)
		}
		uidEnvironment := append(append([]string{}, c.LandingEnvironment...), "PROPOSAL_MAIN="+c.Main)
		if _, err = l.Commands.Run(ctx, c.Main, uidEnvironment, "", filepath.Join(dir, t.ID+"-uids.log"), "bash", uidArgs...); err != nil {
			return result, err
		}
		record.TargetBefore, err = git("rev-parse", "HEAD")
		if err != nil {
			return result, err
		}
		record.Step = "review"
		if err = save(); err != nil {
			return result, err
		}
	}

	if record.Step == "review" {
		acceptancePath := filepath.ToSlash(filepath.Join(t.Work.Folder, "acceptance.json"))
		otherChanges, err := l.Repository.Git(ctx, t.Directory, "status", "--porcelain=v1", "--untracked-files=all", "--", ".", ":(exclude)"+acceptancePath)
		if err != nil {
			return result, err
		}
		if otherChanges != "" {
			return result, fmt.Errorf("slot changed while recording acceptance:\n%s", otherChanges)
		}
		current, err := l.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
		if err != nil {
			return result, err
		}
		subject := "Review: user accepted candidate " + record.Candidate
		if current == record.Candidate {
			data, err := json.MarshalIndent(t.Verdict, "", "  ")
			if err != nil {
				return result, err
			}
			destination := filepath.Join(t.Directory, t.Work.Folder, "acceptance.json")
			if err = os.WriteFile(destination, append(data, '\n'), 0600); err != nil {
				return result, err
			}
			args := []string{}
			if c.LandingReviewHooks == "skip" {
				args = []string{"-c", "core.hooksPath=/dev/null"}
			}
			if _, err = l.Repository.Git(ctx, t.Directory, append(args, "add", "--", filepath.Join(t.Work.Folder, "acceptance.json"))...); err != nil {
				return result, err
			}
			if _, err = l.Commands.Run(ctx, t.Directory, c.LandingEnvironment, "", "", "git", append(args, "commit", "-m", subject+"\n\n"+strings.Join(trailers, "\n"))...); err != nil {
				return result, err
			}
			current, err = l.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
			if err != nil {
				return result, err
			}
		} else {
			parent, err := l.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD^")
			if err != nil {
				return result, err
			}
			actual, err := l.Repository.Git(ctx, t.Directory, "show", "-s", "--format=%s", "HEAD")
			if err != nil {
				return result, err
			}
			if parent != record.Candidate || actual != subject {
				return result, fmt.Errorf("slot moved during acceptance recording")
			}
			paths, err := l.Repository.Changes(ctx, t.Directory, record.Candidate, current)
			if err != nil {
				return result, err
			}
			if len(paths) != 1 || paths[0] != acceptancePath {
				return result, fmt.Errorf("review commit changed more than the acceptance record")
			}
			stored, err := l.Repository.Read(ctx, t.Directory, current, filepath.Join(t.Work.Folder, "acceptance.json"))
			if err != nil {
				return result, err
			}
			var recorded api.Decision
			if err = material.Decode(stored, &recorded); err != nil {
				return result, err
			}
			if !reflect.DeepEqual(recorded, *t.Verdict) {
				return result, fmt.Errorf("committed acceptance differs from the frozen verdict")
			}
		}
		record.Review = current
		record.Step = "merge"
		if err = save(); err != nil {
			return result, err
		}
	}
	if record.Step == "merge" {
		current, err := git("rev-parse", "HEAD")
		if err != nil {
			return result, err
		}
		if current != record.TargetBefore {
			parents, err := git("show", "-s", "--format=%P", "HEAD")
			if err != nil {
				return result, err
			}
			if parents != record.TargetBefore+" "+record.Review {
				return result, fmt.Errorf("main moved during landing")
			}
			record.Merge = current
		} else {
			mergeHead, mergeErr := git("rev-parse", "--verify", "MERGE_HEAD")
			if mergeErr == nil {
				if mergeHead != record.Review {
					return result, fmt.Errorf("another merge is in progress")
				}
			} else {
				if _, err = git("merge", "--no-ff", "--no-commit", record.Review); err != nil {
					return result, fmt.Errorf("landing merge stopped; resolve conflicts, stage them, then resume: %w", err)
				}
			}
			conflicts, err := git("diff", "--name-only", "--diff-filter=U")
			if err != nil {
				return result, err
			}
			if conflicts != "" {
				return result, fmt.Errorf("unresolved landing conflicts:\n%s", conflicts)
			}
			message := "Land proposal " + t.Proposal + ": " + t.Candidate.Title + "\n\nAccepted-Candidate: " + record.Candidate + "\nSigned-By: " + t.Verdict.Signer + "\n" + strings.Join(trailers, "\n")
			if _, err = git("commit", "-m", message); err != nil {
				return result, err
			}
			record.Merge, err = git("rev-parse", "HEAD")
			if err != nil {
				return result, err
			}
		}
		record.Step = "royal-guard"
		if err = save(); err != nil {
			return result, err
		}
	}
	if record.Step == "royal-guard" {
		if _, err = l.Commands.Run(ctx, c.Main, c.LandingEnvironment, "", filepath.Join(dir, t.ID+"-royal-guard.log"), "make", "royal-guard"); err != nil {
			return result, err
		}
		record.Step = "push"
		if err = save(); err != nil {
			return result, err
		}
	}
	if record.Step == "push" {
		if _, err = git("push", "origin", record.Merge+":refs/heads/"+c.TargetBranch); err != nil {
			return result, err
		}
		record.Step = "outcome"
		if err = save(); err != nil {
			return result, err
		}
	}
	if record.Step == "outcome" && t.Phase == "land" {
		result.OK = true
		result.Detail = "landed and pushed " + record.Merge + " (review " + record.Review + "); landed-tree Outcome pending"
		return result, nil
	}
	if record.Step == "outcome" {
		if c.LandingOutcome == "none" {
			record.Step = "complete"
			if err = save(); err != nil {
				return result, err
			}
			result.OK = true
			result.Detail = "landed and pushed " + record.Merge + "; project declares no post-landing Outcome"
			return result, nil
		}
		if err = l.Repository.Clean(ctx, t.Directory); err != nil {
			return result, err
		}
		if _, err = l.Repository.Git(ctx, t.Directory, "checkout", "--detach", record.Merge); err != nil {
			return result, err
		}
		if c.LandingOutcome == "validate-candidate-outcome" {
			if t.Work.Outcome == "" || t.Work.Reviewed == "" {
				return result, fmt.Errorf("accepted candidate has no measured Outcome")
			}
			if err = material.CheckOutcome(ctx, l.Repository, t.Directory, c.Project, t.Work.Outcome, t.Work.Reviewed, false); err != nil {
				return result, err
			}
			record.Outcome = t.Work.Outcome
			record.Step = "complete"
			if err = save(); err != nil {
				return result, err
			}
			result.OK = true
			result.Detail = "landed and pushed " + record.Merge + "; candidate Outcome validated at " + t.Work.Reviewed
			return result, nil
		}
		record.Outcome = filepath.Join(dir, t.ID+"-outcome")
		_, err = l.Commands.Run(ctx, t.Directory, append(append([]string{}, c.LandingEnvironment...), "OUTCOME_BASE="+record.TargetBefore, "OUTCOME_LOCK="+filepath.Join(c.State, "outcome.lock")), "", filepath.Join(dir, t.ID+"-outcome.log"), "bash", filepath.Join(t.Directory, "projects/common/outcome/run.sh"), filepath.Join(t.Directory, c.Project), record.Outcome)
		if err != nil {
			return result, fmt.Errorf("merged and pushed %s; landed-tree Outcome failed: %w", record.Merge, err)
		}
		if err = material.CheckOutcome(ctx, l.Repository, t.Directory, c.Project, record.Outcome, record.Merge, false); err != nil {
			return result, err
		}
		result.Documents, err = material.OutcomeDocuments(record.Outcome, record.Merge)
		if err != nil {
			return result, err
		}
		record.Step = "complete"
		if err = save(); err != nil {
			return result, err
		}
	}
	result.OK = true
	result.Detail = "landed and pushed " + record.Merge + "; landed-tree Outcome " + record.Outcome
	return result, nil
}
