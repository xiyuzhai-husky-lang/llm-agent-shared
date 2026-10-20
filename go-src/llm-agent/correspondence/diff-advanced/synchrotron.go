package main

// The diff/advanced synchrotron: the state one connected page and the server agree on, and the single append-only
// action log both sides replay to reach it.
//
// The server holds one synchrotron per connection. A page opens with `init`, which the server answers with the actions
// that take an empty synchrotron to the range's header and view; every later view action the page cannot answer from
// its own synchrotron goes up with the number of actions the page holds, and the server answers with the actions the
// page has not seen.

import (
	"encoding/json"
	"fmt"
	"slices"

	"diff-advanced/tab"
)

// header is what the title bar and Peek show of the range.
type header struct {
	Title     string `json:"title"`
	Selection string `json:"selection"`
	Base      string `json:"base"`
	Candidate string `json:"candidate"`
	BaseShort string `json:"base_short"`
	// CandidateShort is the candidate's short commit, or `working tree`.
	CandidateShort string `json:"candidate_short"`
	// ChangedPaths is every changed path of the range, in the order the index lists them.
	ChangedPaths []string `json:"changed_paths"`
}

type queuedPeekQuestion struct {
	Identifier string `json:"identifier"`
	Text       string `json:"text"`
}

// action is one change to the synchrotron, written `{"kind": ..., ...}`.
type action struct {
	Kind       string              `json:"kind"`
	Header     *header             `json:"header,omitempty"`
	View       *tab.View           `json:"view,omitempty"`
	Query      *tab.Query          `json:"query,omitempty"`
	Answer     *tab.Answer         `json:"answer,omitempty"`
	Question   *queuedPeekQuestion `json:"question,omitempty"`
	Identifier *string             `json:"identifier,omitempty"`
}

type status struct {
	ActionsLen uint64 `json:"actions_len"`
}

type actionsDiff struct {
	From       status   `json:"from"`
	NewActions []action `json:"new_actions"`
}

type heldAnswer struct {
	query  *tab.Query
	answer tab.Answer
}

// synchrotron is the state one page and the server agree on.
type synchrotron struct {
	header        *header
	view          *tab.View
	answers       map[string]heldAnswer
	peekQuestions []queuedPeekQuestion
	actions       []action
}

func newSynchrotron() *synchrotron { return &synchrotron{answers: map[string]heldAnswer{}} }

func (mirror *synchrotron) status() status { return status{ActionsLen: uint64(len(mirror.actions))} }

// answer is the held answer to `query` when it was answered at `minimumGeneration` or later.
func (mirror *synchrotron) answer(query *tab.Query, minimumGeneration uint64) bool {
	held, known := mirror.answers[query.Key()]
	if !known {
		return false
	}
	generation, versioned := held.answer.Generation()
	return !versioned || generation >= minimumGeneration
}

// signatureDependentQueries is the held queries a signature changes the answers of, in query order.
func (mirror *synchrotron) signatureDependentQueries() []*tab.Query {
	var queries []*tab.Query
	for _, held := range mirror.answers {
		if held.query.SignatureDependent() {
			queries = append(queries, held.query)
		}
	}
	slices.SortFunc(queries, tab.CompareQueries)
	return queries
}

func (mirror *synchrotron) actionsDiff(from status) actionsDiff {
	return actionsDiff{From: from, NewActions: slices.Clone(mirror.actions[from.ActionsLen:])}
}

func (mirror *synchrotron) takeAction(taken action) error {
	switch taken.Kind {
	case "initialize":
		if len(mirror.actions) > 0 {
			return fmt.Errorf("Initialize must be the first action")
		}
		mirror.header, mirror.view = taken.Header, taken.View
	case "cache_answer":
		if mirror.view == nil {
			return fmt.Errorf("an answer arrived before Initialize")
		}
		mirror.answers[taken.Query.Key()] = heldAnswer{query: taken.Query, answer: *taken.Answer}
	case "replace_view":
		if mirror.view == nil {
			return fmt.Errorf("ReplaceView before Initialize")
		}
		if taken.View.Generation <= mirror.view.Generation {
			return fmt.Errorf("a replaced view must carry a newer generation")
		}
		mirror.view = taken.View
	case "queue_peek_question":
		if slices.ContainsFunc(mirror.peekQuestions, func(queued queuedPeekQuestion) bool { return queued.Identifier == taken.Question.Identifier }) {
			return fmt.Errorf("Peek question %s is already queued", taken.Question.Identifier)
		}
		mirror.peekQuestions = append(mirror.peekQuestions, *taken.Question)
	case "consume_peek_question":
		index := slices.IndexFunc(mirror.peekQuestions, func(queued queuedPeekQuestion) bool { return queued.Identifier == *taken.Identifier })
		if index < 0 {
			return fmt.Errorf("Peek question %s is not queued", *taken.Identifier)
		}
		mirror.peekQuestions = slices.Delete(mirror.peekQuestions, index, index+1)
	default:
		panic(fmt.Sprintf("unknown synchrotron action %q", taken.Kind))
	}
	mirror.actions = append(mirror.actions, taken)
	return nil
}

// viewAction is what the page asks of the server once it holds the synchrotron.
type viewAction struct {
	Kind       string      `json:"kind"`
	Query      *tab.Query  `json:"query"`
	Action     *tab.Action `json:"action"`
	Identifier *string     `json:"identifier"`
}

// request is `{"kind": "init", "request_identifier"}` or `{"kind": "take_view_action", "request_identifier",
// "view_action", "status"}`.
type request struct {
	Kind              string      `json:"kind"`
	RequestIdentifier uint64      `json:"request_identifier"`
	ViewAction        *viewAction `json:"view_action"`
	Status            *status     `json:"status"`
}

func parseRequest(text []byte) (*request, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(text, &fields); err != nil {
		return nil, err
	}
	var kind string
	if err := json.Unmarshal(fields["kind"], &kind); err != nil {
		return nil, fmt.Errorf("the request carries no kind")
	}
	required := map[string][]string{"init": {"kind", "request_identifier"},
		"take_view_action": {"kind", "request_identifier", "view_action", "status"}}[kind]
	if required == nil {
		return nil, fmt.Errorf("unknown request kind %q", kind)
	}
	if len(fields) != len(required) {
		return nil, fmt.Errorf("a %s request carries exactly %v", kind, required)
	}
	for _, field := range required {
		if _, present := fields[field]; !present {
			return nil, fmt.Errorf("missing field `%s`", field)
		}
	}
	var parsed request
	if err := json.Unmarshal(text, &parsed); err != nil {
		return nil, err
	}
	if parsed.ViewAction != nil {
		var viewFields map[string]json.RawMessage
		if err := json.Unmarshal(fields["view_action"], &viewFields); err != nil {
			return nil, err
		}
		field := map[string]string{"ask": "query", "act": "action", "consume_peek_question": "identifier"}[parsed.ViewAction.Kind]
		if _, present := viewFields[field]; field == "" || !present || len(viewFields) != 2 {
			return nil, fmt.Errorf("the view action %s is not a diff/advanced view action", fields["view_action"])
		}
	}
	return &parsed, nil
}

type response struct {
	Kind              string       `json:"kind"`
	RequestIdentifier uint64       `json:"request_identifier"`
	Diff              *actionsDiff `json:"diff,omitempty"`
	Message           *string      `json:"message,omitempty"`
}
