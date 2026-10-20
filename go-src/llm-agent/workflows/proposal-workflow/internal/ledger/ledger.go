package ledger

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Ledger owns indexed batches, membership and the active batch stack.
// Loading policy: The workflow service restores batch membership from journal events.
// Caching policy: Keep indexed batches, members and stack order; Plan returns a changed copy without mutating the original.
type Ledger struct {
	Batches map[string]*Batch  `json:"batches"`
	Members map[string]*Member `json:"members"`
	Stack   []string           `json:"stack"`
	Next    uint64             `json:"next"`
}

func New() *Ledger {
	return &Ledger{Batches: map[string]*Batch{}, Members: map[string]*Member{}, Stack: []string{}, Next: 1}
}

func (l *Ledger) Plan(a Action) (*Ledger, error) {
	data, _ := json.Marshal(l)
	next := New()
	if err := json.Unmarshal(data, next); err != nil {
		return nil, err
	}
	if err := next.apply(a); err != nil {
		return nil, err
	}
	return next, nil
}

func (l *Ledger) apply(a Action) error {
	switch a.Kind {
	case "open":
		if a.Batch == "" {
			a.Batch = strconv.FormatUint(l.Next, 10)
		}
		if number, err := strconv.ParseUint(a.Batch, 10, 64); err == nil && number >= l.Next {
			if number >= 4294967295 {
				return fmt.Errorf("batch number cannot leave a u32 next")
			}
			l.Next = number + 1
		}
		if a.Batch == "" || l.Batches[a.Batch] != nil {
			return fmt.Errorf("batch ID must be new")
		}
		l.Batches[a.Batch] = &Batch{ID: a.Batch, Title: a.Title, Description: a.Scope, StartedAt: a.StartedAt, Members: []string{}}
		l.Stack = append([]string{a.Batch}, l.Stack...)
	case "add":
		if a.Batch == "" && len(l.Stack) > 0 {
			a.Batch = l.Stack[0]
		}
		b := l.Batches[a.Batch]
		if b == nil || b.Closed || a.UID == "" || l.Members[a.UID] != nil {
			return fmt.Errorf("invalid batch membership")
		}
		ordinal := uint64(1)
		for _, uid := range b.Members {
			if l.Members[uid].Ordinal >= ordinal {
				ordinal = l.Members[uid].Ordinal + 1
			}
		}
		if ordinal > 4294967295 {
			return fmt.Errorf("ordinal exceeds u32")
		}
		l.Members[a.UID] = &Member{Ordinal: ordinal, OriginBatch: a.OriginBatch, UID: a.UID, Batch: a.Batch, Title: a.Title, Scope: a.Scope, Branch: a.Branch}
		b.Members = append(b.Members, a.UID)
	case "approve":
		b := l.Batches[a.Batch]
		if b == nil || b.Closed || a.Source == "" {
			return fmt.Errorf("approval requires an open batch and source")
		}
		b.Approval = a.Source
		b.ApprovedAt = a.ApprovedAt
		b.ApprovalText = a.ApprovalText
	case "dispatch":
		m := l.Members[a.UID]
		if m == nil || a.Branch == "" || (m.Branch != "" && m.Branch != a.Branch) {
			return fmt.Errorf("dispatch requires a member and an unchanged or new branch")
		}
		m.Branch = a.Branch
	case "edit":
		b := l.Batches[a.Batch]
		if b == nil || b.Closed || (a.Title == "" && a.Scope == "") {
			return fmt.Errorf("edit requires an open batch and title or description")
		}
		if a.Title != "" {
			b.Title = a.Title
		}
		if a.Scope != "" {
			b.Description = a.Scope
		}
	case "drop":
		b := l.Batches[a.Batch]
		if b == nil || b.Closed || b.Approval != "" || len(b.Members) != 0 {
			return fmt.Errorf("drop requires an empty unapproved open batch")
		}
		delete(l.Batches, a.Batch)
		for i, id := range l.Stack {
			if id == a.Batch {
				l.Stack = append(l.Stack[:i], l.Stack[i+1:]...)
				break
			}
		}
	case "withdraw":
		m := l.Members[a.UID]
		if m == nil || (m.Settlement != "" && m.Settlement != "abandoned") {
			return fmt.Errorf("only unworked or abandoned membership can be withdrawn")
		}
		b := l.Batches[m.Batch]
		for i, uid := range b.Members {
			if uid == a.UID {
				b.Members = append(b.Members[:i], b.Members[i+1:]...)
				break
			}
		}
		delete(l.Members, a.UID)
		if len(b.Members) == 0 {
			delete(l.Batches, b.ID)
			if number, err := strconv.ParseUint(b.ID, 10, 64); err == nil && number == l.Next-1 {
				l.Next = 1
				for id := range l.Batches {
					if n, err := strconv.ParseUint(id, 10, 64); err == nil && n >= l.Next {
						l.Next = n + 1
					}
				}
			}
			for i, id := range l.Stack {
				if id == b.ID {
					l.Stack = append(l.Stack[:i], l.Stack[i+1:]...)
					break
				}
			}
		}
	case "settle":
		m := l.Members[a.UID]
		if m == nil || m.Settlement != "" {
			return fmt.Errorf("settlement requires an unsettled member")
		}
		value := a.Value
		switch value {
		case "landed", "rejected", "abandoned", "retired", "transferred":
		default:
			return fmt.Errorf("invalid settlement")
		}
		m.Settlement = value
		m.Source = a.Source
	case "transfer":
		m := l.Members[a.UID]
		b := l.Batches[a.Batch]
		if m == nil || m.Settlement != "" || b == nil || b.Closed || m.Batch == a.Batch {
			return fmt.Errorf("invalid transfer")
		}
		old := l.Batches[m.Batch]
		for i, id := range old.Members {
			if id == m.UID {
				old.Members = append(old.Members[:i], old.Members[i+1:]...)
				break
			}
		}
		m.Batch = a.Batch
		m.Ordinal = 1
		for _, uid := range b.Members {
			if l.Members[uid].Ordinal >= m.Ordinal {
				m.Ordinal = l.Members[uid].Ordinal + 1
			}
		}
		b.Members = append(b.Members, m.UID)
	case "close":
		b := l.Batches[a.Batch]
		if b == nil || b.Closed || len(l.Stack) == 0 || l.Stack[0] != a.Batch {
			return fmt.Errorf("only the top open batch can close")
		}
		for _, id := range b.Members {
			if l.Members[id].Settlement == "" {
				return fmt.Errorf("batch has unsettled members")
			}
		}
		b.Closed = true
		l.Stack = l.Stack[1:]
	default:
		return fmt.Errorf("unknown ledger action %q", a.Kind)
	}
	return nil
}

func (l *Ledger) CanStart(uid string) error {
	m := l.Members[uid]
	if m == nil || m.Settlement != "" {
		return fmt.Errorf("proposal is not an unsettled member")
	}
	b := l.Batches[m.Batch]
	if b.Closed || b.Approval == "" || len(l.Stack) == 0 || l.Stack[0] != b.ID {
		return fmt.Errorf("proposal must belong to the approved top batch")
	}
	return nil
}

// Validate checks the journal projection without changing approval or settlement.
func (l *Ledger) Validate() error {
	if l.Next == 0 || l.Next > 4294967295 {
		return fmt.Errorf("invalid next batch number")
	}
	stack := map[string]bool{}
	for _, id := range l.Stack {
		if stack[id] || l.Batches[id] == nil || l.Batches[id].Closed {
			return fmt.Errorf("invalid batch stack entry %q", id)
		}
		stack[id] = true
	}
	members := map[string]bool{}
	for id, batch := range l.Batches {
		if number, err := strconv.ParseUint(id, 10, 64); err == nil && number >= l.Next {
			return fmt.Errorf("batch %s is not below next", id)
		}
		ordinals := map[uint64]bool{}

		if batch == nil || batch.ID != id || (!batch.Closed && !stack[id]) {
			return fmt.Errorf("invalid batch %q", id)
		}
		for _, uid := range batch.Members {
			member := l.Members[uid]
			if members[uid] || member == nil || member.UID != uid || member.Batch != id {
				return fmt.Errorf("invalid membership %q", uid)
			}
			if member.Ordinal == 0 || ordinals[member.Ordinal] {
				return fmt.Errorf("invalid ordinal for %s", uid)
			}
			ordinals[member.Ordinal] = true
			switch member.Settlement {
			case "", "landed", "rejected", "abandoned", "retired", "transferred":
			default:
				return fmt.Errorf("invalid settlement for %s", uid)
			}
			if batch.Closed && member.Settlement == "" {
				return fmt.Errorf("closed batch %s contains unsettled %s", id, uid)
			}
			members[uid] = true
		}
	}
	for uid := range l.Members {
		if !members[uid] {
			return fmt.Errorf("unlisted member %s", uid)
		}
	}
	return nil
}

// CloseSettledTop removes one completed top frame, leaving suspended frames alone.
func (l *Ledger) CloseSettledTop() {
	if len(l.Stack) == 0 {
		return
	}
	b := l.Batches[l.Stack[0]]
	if len(b.Members) == 0 {
		return
	}
	for _, uid := range b.Members {
		if l.Members[uid].Settlement == "" {
			return
		}
	}
	b.Closed = true
	l.Stack = l.Stack[1:]
}
