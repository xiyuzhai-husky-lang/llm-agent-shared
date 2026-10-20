package execution

import (
	"context"
	"fmt"

	"proposal-platform/api"
	"proposal-workflow/internal/workflow"
)

func (p *Project) role(ctx context.Context, t workflow.Task) (workflow.Result, error) {
	before, err := p.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
	if err != nil {
		return workflow.Result{}, err
	}
	if t.Phase != "worker" && t.Phase != "wrapup" {
		if t.Candidate == nil || before != t.Candidate.ID {
			return workflow.Result{}, fmt.Errorf("%s requires the pinned candidate HEAD", t.Phase)
		}
		if err = p.Repository.Clean(ctx, t.Directory); err != nil {
			return workflow.Result{}, err
		}
	}
	role := t.Phase
	if role == "response" || role == "wrapup" {
		role = "worker"
	}
	work := t.Work
	if work.Sessions == nil {
		work.Sessions = map[string]string{}
	}
	if role == "commissar" {
		work.Sessions[role] = ""
	}
	report, result, err := p.readReport(ctx, t, role, &work)
	if err != nil {
		return result, err
	}
	after, err := p.Repository.Git(ctx, t.Directory, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	if err = p.Repository.Clean(ctx, t.Directory); err != nil {
		return result, err
	}
	result.Changed = before != after
	result.Detail = report.Text
	if role != "worker" || t.Phase == "response" {
		if before != after {
			return result, fmt.Errorf("%s changed the read-only candidate", t.Phase)
		}
	}
	if t.Phase == "response" {
		result.Answers = report.Answers
		result.OK = true
		return result, nil
	}
	if t.Phase == "wrapup" {
		candidate := *t.Candidate
		candidate.ID = after
		if candidate.Source != nil {
			source := *candidate.Source
			source.Commit = after
			candidate.Source = &source
		}
		result.Candidate = &candidate

		result.OK = true
		return result, nil
	}
	if role == "worker" {
		work.Question = report.Step.Question
		work.Problems = report.Step.Problems
		work.Present = report.Present
		result.Answers = report.Answers
		result.Present = report.Present
		if t.Candidate != nil && before == after && t.Purpose != "revision" {
			result.Candidate = t.Candidate
		} else {
			candidate := &api.Candidate{Proposal: t.Proposal, ID: after, Title: t.Title, Kind: report.Step.Kind, Base: t.Work.Base}
			result.Candidate = candidate
			work.Reviewed = after
			work.Outcome = ""
		}
		if result.Candidate.Kind != report.Step.Kind {
			return result, fmt.Errorf("unchanged candidate has a different step kind")
		}
	} else {
		result.Halt = report.Halt
	}
	if t.Phase == "worker" && t.Purpose == "feedback" && before == after && len(result.Answers) != len(t.Feedback) {
		return result, fmt.Errorf("missing Worker answers")
	}
	result.OK = true
	return result, nil
}
