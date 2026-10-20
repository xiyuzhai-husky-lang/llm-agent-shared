package workflow

import (
	"proposal-platform/api"
	"time"
)

// Decide consumes an immutable feed fact; historical candidate decisions do not restart work.
func Decide(current *Proposal, d api.Decision) *Proposal {
	p := current.Clone()
	if p.Candidate == nil || p.Candidate.ID != d.Candidate || d.Kind != "verdict" {
		return p
	}
	switch d.Value {
	case "accept":
		p.Status = "accepted"
		p.Verdict = &d
		p.Task = nil
		p.Generation++
	case "reject":
		p.Status = "rejected"
		p.Verdict = &d
		p.Task = nil
		p.Generation++
	case "interact":
		p.Status = "interacting"
		p.Verdict = &d
		p.Task = nil
		p.Generation++
	case "modify":
		p.Work.Parent = d.Candidate
		p.Work.Reviewed = ""
		p.Work.Outcome = ""
		p.Status = "running"
		p.Phase = "preflight"
		p.Purpose = "revision"
		p.Verdict = &d
		p.Task = nil
		p.Generation++
		p.Revision++
		p.DeadlineExceeded = false
		if at, err := time.Parse(time.RFC3339Nano, d.At); err == nil {
			p.StartedAt = at.Unix()
		}
		p.WorkerRevisions = 0
		p.GateFailures = 0
		p.WrapFailures = 0
		p.UserText = d.Text
		p.Feedback = append([]api.Doubt(nil), d.Sustained...)
	case "revoke":
		if p.Verdict != nil && p.Verdict.Value == "modify" && p.Status == "running" {
			p.Status = "stopped"
			p.Reason = "Modify was revoked; explicit resume required"
		} else if p.Verdict != nil {
			p.Status = "present"
			p.Phase = ""
		}
		p.Verdict = nil
		p.Task = nil
		p.Generation++
	}
	return p
}
