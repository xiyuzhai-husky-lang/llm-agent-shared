package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/workflow"
)

func label(value string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`).MatchString(value)
}

func (s *Service) Submit(c Command) (api.Receipt, error) {
	if !label(c.ID) {
		return api.Receipt{}, fmt.Errorf("invalid command ID")
	}
	digest, err := journal.Digest(c)
	if err != nil {
		return api.Receipt{}, err
	}
	if c.RequestDigest != "" {
		digest = c.RequestDigest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if receipt, found, err := s.duplicate("command/"+c.ID, digest); found || err != nil {
		return receipt, err
	}
	e := Event{Kind: c.Action, Proposals: map[string]*workflow.Proposal{}, Slots: map[string]string{}, Keys: []string{}}
	if c.Action == "connect_decisions" {
		if err = s.connectDecisions(c, &e); err != nil {
			return api.Receipt{}, err
		}
	} else if c.Action == "ledger" {
		if err = s.planLedger(c, &e); err != nil {
			return api.Receipt{}, err
		}
	} else {
		if !label(c.UID) {
			return api.Receipt{}, fmt.Errorf("invalid proposal ID")
		}
		current := s.proposals[c.UID]
		var p *workflow.Proposal
		if c.Action == "start" {
			if current != nil {
				return api.Receipt{}, fmt.Errorf("proposal already started; use resume")
			}
			if err = s.ledger.CanStart(c.UID); err != nil {
				return api.Receipt{}, err
			}
			if !label(c.Slot) || s.slots[c.Slot] != "" {
				return api.Receipt{}, fmt.Errorf("slot is invalid or occupied")
			}
			if c.Directory != "" {
				for _, uid := range s.slots {
					if s.proposals[uid].Directory == c.Directory {
						return api.Receipt{}, fmt.Errorf("directory is reserved by %s", uid)
					}
				}
			}
			policy, exists := s.policies[c.Rank]
			if !exists {
				return api.Receipt{}, fmt.Errorf("unknown mission rank %s", c.Rank)
			}
			p = &workflow.Proposal{Orchestrator: c.Orchestrator, UID: c.UID, Batch: s.ledger.Members[c.UID].Batch, Slot: c.Slot, Directory: c.Directory, Assignment: c.Assignment, Work: workflow.Work{Folder: c.Folder, Base: c.Base, Sessions: map[string]string{}}, Policy: policy, Status: "running", Phase: "preflight", Purpose: "initial", Revision: 1, StartedAt: time.Now().Unix()}
			e.Slots[c.Slot] = c.UID
			e.Keys = append(e.Keys, "slot/"+c.Slot)
		} else {
			if current == nil {
				return api.Receipt{}, fmt.Errorf("unknown proposal")
			}
			p = current.Clone()
			if c.Slot != "" && c.Slot != p.Slot {
				return api.Receipt{}, fmt.Errorf("slot differs from proposal binding")
			}
			switch c.Action {
			case "reset", "restart":
				if p.Status == "landed" || p.Status == "settled" || p.Phase == "land" || p.Phase == "land-outcome" {
					return api.Receipt{}, fmt.Errorf("cannot reset a landing or settled proposal")
				}
				if err = s.ledger.CanStart(c.UID); err != nil {
					return api.Receipt{}, err
				}
				status := "stopped"
				if c.Action == "restart" {
					status = "running"
				}
				p = &workflow.Proposal{Orchestrator: current.Orchestrator, UID: current.UID, Batch: current.Batch, Slot: current.Slot, Directory: current.Directory, Assignment: current.Assignment, Policy: current.Policy, Work: workflow.Work{Folder: current.Work.Folder, Sessions: map[string]string{}}, Status: status, Phase: "preflight", Purpose: "initial", Revision: 1, Generation: current.Generation + 1, StartedAt: time.Now().Unix()}
			case "retire":
				if c.Kind != "rejected" && c.Kind != "abandoned" && c.Kind != "retired" {
					return api.Receipt{}, fmt.Errorf("end-early kind must be rejected, abandoned or retired")
				}
				if err := s.ledger.CanStart(p.UID); err != nil {
					return api.Receipt{}, err
				}
				if p.Phase == "land" || p.Phase == "land-outcome" {
					return api.Receipt{}, fmt.Errorf("cannot end a landing early")
				}
				p.Status = "running"
				p.Phase = "retire-check"
				p.Purpose = c.Kind
				p.UserText = c.Source
				p.Task = nil
				p.Generation++
			case "land":
				if p.Status != "accepted" || p.Verdict == nil || p.Verdict.Value != "accept" {
					return api.Receipt{}, fmt.Errorf("landing requires standing Accept")
				}
				p.Status = "running"
				p.Phase = "land"
				p.Task = nil
				p.Generation++
			case "stop":
				if p.Status != "running" {
					return api.Receipt{}, fmt.Errorf("proposal is not running")
				}
				p.Status = "stopped"
				p.Task = nil
				p.Generation++
			case "resume":
				switch p.Status {
				case "stopped", "stuck", "preflight-failed", "delivery-failed", "aborted":
				default:
					return api.Receipt{}, fmt.Errorf("proposal cannot resume from %s", p.Status)
				}
				if p.Phase != "land-outcome" && p.Phase != "retire-clean" {
					if err = s.ledger.CanStart(c.UID); err != nil {
						return api.Receipt{}, err
					}
				}
				if p.Phase == "" {
					return api.Receipt{}, fmt.Errorf("no phase to resume")
				}
				p.Status = "running"
				p.Task = nil
				p.Generation++
				p.Reason = "Explicit resume: inspect unfinished external effects before continuing"
				if c.Base != "" {
					if p.Phase == "land" || p.Phase == "land-outcome" {
						return api.Receipt{}, fmt.Errorf("cannot change a landing base")
					}
					p.Work.Base = c.Base
					if p.Verdict != nil {
						p.Work.Parent = p.Verdict.Candidate
					}
					p.Phase = "preflight"
					p.Purpose = "revision"
				}
				if c.Rank != "" {
					policy, ok := s.policies[c.Rank]
					if !ok {
						return api.Receipt{}, fmt.Errorf("unknown mission rank %s", c.Rank)
					}
					p.Policy = policy
				}
			case "end_early":
				if p.Status != "running" && p.Status != "stopped" {
					return api.Receipt{}, fmt.Errorf("proposal cannot end early")
				}
				p.Task = nil
				p.Generation++
				if p.Candidate == nil {
					p.Status = "aborted"
					p.Reason = "ended before a candidate existed"
				} else {
					p.Status = "running"
					p.Phase = "wrapup"
					p.Purpose = "end-early"
				}
			case "republish":
				if p.Status != "present" || p.Candidate == nil {
					return api.Receipt{}, fmt.Errorf("only a presented candidate can be republished")
				}
				p.Status = "running"
				p.Phase = "deliver"
				p.Task = nil
				p.Generation++
			case "answer":
				if p.Status != "present" || p.Candidate == nil || p.Candidate.Kind != "incomplete" {
					return api.Receipt{}, fmt.Errorf("only a presented incomplete candidate can be answered")
				}
				if strings.TrimSpace(c.Source) == "" {
					return api.Receipt{}, fmt.Errorf("answer requires --source with the user's answer")
				}
				p.Work.Parent = p.Candidate.ID
				p.Work.Reviewed = ""
				p.Work.Outcome = ""
				p.Status = "running"
				p.Phase = "preflight"
				p.Purpose = "revision"
				p.Task = nil
				p.Generation++
				p.Revision++
				p.DeadlineExceeded = false
				p.StartedAt = time.Now().Unix()
				p.WorkerRevisions = 0
				p.GateFailures = 0
				p.WrapFailures = 0
				p.UserText = c.Source
				p.Feedback = nil
			default:
				return api.Receipt{}, fmt.Errorf("unknown workflow command %q", c.Action)
			}
		}
		e.Proposals[c.UID] = p
		e.Keys = append(e.Keys, "proposal/"+c.UID, "event/"+c.UID+"/"+c.ID)
	}
	return s.commit("command/"+c.ID, digest, e)
}

func (s *Service) planLedger(c Command, e *Event) error {
	if c.Ledger == nil {
		return fmt.Errorf("ledger action required")
	}
	a := *c.Ledger
	if (a.Batch != "" && !label(a.Batch)) || (a.UID != "" && !label(a.UID)) {
		return fmt.Errorf("invalid ledger identity")
	}
	current := s.proposals[a.UID]
	if a.Kind == "settle" {
		member := s.ledger.Members[a.UID]
		if member == nil || len(s.ledger.Stack) == 0 || member.Batch != s.ledger.Stack[0] {
			return fmt.Errorf("only an unsettled top-batch member may settle")
		}
	}
	if a.Kind == "withdraw" && current != nil && current.Status != "settled" {
		return fmt.Errorf("running or completed work must be settled before withdrawal")
	}
	if a.Kind == "settle" && a.Value == "landed" && (current == nil || current.Status != "landed") {
		return fmt.Errorf("landed settlement requires completed landing")
	}
	if a.Kind == "transfer" && current != nil {
		return fmt.Errorf("transfer is only available before execution starts")
	}
	if current != nil && (a.Kind == "settle" || a.Kind == "withdraw") {
		if a.Value == "landed" && current.Status != "landed" {
			return fmt.Errorf("landed settlement requires completed landing")
		}
		p := current.Clone()
		p.Status = "settled"
		p.Task = nil
		p.Generation++
		e.Proposals[p.UID] = p
		e.Slots[p.Slot] = ""
		e.Keys = append(e.Keys, "proposal/"+p.UID, "slot/"+p.Slot)
	}
	next, err := s.ledger.Plan(a)
	if err != nil {
		return err
	}
	if a.Kind == "settle" {
		next.CloseSettledTop()
	}
	e.Ledger = next
	e.Keys = append(e.Keys, "ledger")
	return nil
}
