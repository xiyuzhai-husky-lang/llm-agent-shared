package execution

import (
	"context"
	"fmt"

	"proposal-platform/api"
	"proposal-workflow/internal/roles"
	"proposal-workflow/internal/workflow"
)

// readReport corrects protocol/Chow errors in the same role session, at most three times.
func (p *Project) readReport(ctx context.Context, t workflow.Task, role string, work *workflow.Work) (roles.Report, workflow.Result, error) {
	result := workflow.Result{Work: work}
	prompt := roles.Prompt(t, p.Config, role)
	for attempt := 0; attempt <= 3; attempt++ {
		invocation := t
		invocation.Work = *work
		if attempt > 0 {
			invocation.ID = fmt.Sprintf("%s-report-repair-%d", t.ID, attempt)
		}
		text, session, err := p.Roles.Run(ctx, invocation, role, prompt)
		work.Sessions[role] = session
		if err != nil {
			return roles.Report{}, result, err
		}
		result.Detail = text
		if t.Phase == "wrapup" {
			return roles.Report{Text: text}, result, nil
		}
		var report roles.Report
		if t.Phase == "response" {
			report.Text = text
			report.Answers, err = roles.ParseAnswers(text, t.Feedback)
		} else {
			report, err = roles.Parse(text, role, t.ID, t.Feedback)
		}
		if err == nil && role != "worker" {
			result.Findings, err = p.Materials.Doubts(ctx, t, report.Findings)
			if err == nil {
				result.Documents = []api.Document{reportDocument(t, role, text)}
				for i, conflict := range report.Conflicts {
					docs, convertErr := p.Materials.Doubts(ctx, t, []api.Doubt{{ID: fmt.Sprintf("%s-conflict-%d", t.ID, i+1), Text: conflict}})
					if convertErr != nil {
						err = convertErr
						break
					}
					doc := *docs[0].Document
					doc.Kind = "rule-conflict"
					doc.Title = "Rule conflict"
					result.Documents = append(result.Documents, doc)
				}
			}
		}
		if err == nil {
			return report, result, nil
		}
		if attempt == 3 {
			result.Abort = true
			return report, result, fmt.Errorf("role report refused after three corrections: %w", err)
		}
		prompt = "Your previous report could not be read or rendered. Correct the report in this same turn, without changing the worktree. Preserve the original findings and their meaning. Diagnostics:\n" + err.Error()
	}
	panic("unreachable report correction state")
}
