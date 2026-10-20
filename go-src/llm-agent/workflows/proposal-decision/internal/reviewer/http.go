package reviewer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"proposal-decision/internal/service"
	"proposal-platform/httpjson"
	"strings"
	"time"
)

func (s *Server) Register(mux *http.ServeMux) {
	s.registerProjectPage(mux)
	mux.HandleFunc("GET /api/reviewer/workflow-events", s.workflowEvents)
	mux.HandleFunc("GET /api/reviewer", func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := json.Unmarshal([]byte(r.URL.Query().Get("request")), &req); err != nil {
			httpjson.Error(w, err)
			return
		}
		view, err := s.View(req)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, view)
	})
	mux.HandleFunc("GET /api/reviewer/identity", func(w http.ResponseWriter, r *http.Request) {
		var sub Subject
		if err := json.Unmarshal([]byte(r.URL.Query().Get("subject")), &sub); err != nil {
			httpjson.Error(w, err)
			return
		}
		c, state, _, err := s.selection(sub)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, object{"proposal": c.Material.Proposal, "candidate": c.Material.ID, "label": c.Label + " · " + short(c.Material.ID), "draft_allowed": state.Latest && state.Verdict == nil})
	})
	mux.HandleFunc("GET /api/reviewer/resolve", func(w http.ResponseWriter, r *http.Request) {
		uid := r.URL.Query().Get("proposal")
		for _, c := range s.latest() {
			if c.Material.Proposal == uid || fmt.Sprint(c.Subject.Number) == uid {
				httpjson.Write(w, 200, object{"number": c.Subject.Number, "candidate": c.Material.ID})
				return
			}
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("GET /api/reviewer/hunk", func(w http.ResponseWriter, r *http.Request) {
		var sub Subject
		if err := json.Unmarshal([]byte(r.URL.Query().Get("subject")), &sub); err != nil {
			httpjson.Error(w, err)
			return
		}
		text, err := s.HunkText(sub, r.URL.Query().Get("path"), r.URL.Query().Get("hunk"))
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, object{"text": text})
	})
	mux.HandleFunc("POST /api/reviewer/actions", func(w http.ResponseWriter, r *http.Request) {
		var input ActionRequest
		if err := httpjson.Decode(w, r, &input); err != nil {
			httpjson.Error(w, err)
			return
		}
		view, err := s.Act(input)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, view)
	})
	mux.HandleFunc("GET /candidate-file", s.file)
	mux.HandleFunc("GET /kathy-source", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("oid") + ":" + r.URL.Query().Get("path")
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, c := range s.candidates {
			if text, ok := c.Sources[key]; ok {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				fmt.Fprint(w, text)
				return
			}
		}
		http.NotFound(w, r)
	})
}
func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.Unmarshal([]byte(r.URL.Query().Get("request")), &req); err != nil {
		httpjson.Error(w, err)
		return
	}
	name, sub, err := decodeRequest(req)
	if err != nil {
		httpjson.Error(w, err)
		return
	}
	c, state, snap, err := s.selection(sub)
	if err != nil {
		httpjson.Error(w, err)
		return
	}
	if name != "DiffTab" {
		text, err := s.Source(r.Context(), sub, r.URL.Query().Get("path"))
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, text)
		return
	}

	var query struct {
		Children *struct {
			Group     string `json:"group"`
			Directory string `json:"directory"`
		} `json:"children"`
		File *struct {
			Path  string    `json:"path"`
			Cited *Citation `json:"cited"`
		} `json:"file"`
		Document *struct {
			Path string `json:"path"`
		} `json:"document"`
	}
	if err = json.Unmarshal([]byte(r.URL.Query().Get("path")), &query); err != nil {
		httpjson.Error(w, err)
		return
	}
	if query.Children != nil {
		httpjson.Write(w, 200, object{"generation": snap.Sequence, "nodes": c.nodes(query.Children.Group, query.Children.Directory, state)})
		return
	}
	p := ""
	if query.File != nil {
		p = query.File.Path
	}
	if query.Document != nil {
		p = query.Document.Path
	}
	f := c.Files[p]
	if f == nil {
		httpjson.Error(w, fmt.Errorf("file is not in this candidate: %s", p))
		return
	}
	if query.Document != nil {
		httpjson.Write(w, 200, object{"path": p, "notice": nil, "body_notice": f.Notice, "blocks": f.Blocks})
		return
	}
	httpjson.Write(w, 200, c.fileView(f, state, snap, query.File.Cited))
}

type ActionRequest struct {
	ID      string                     `json:"id"`
	Request Request                    `json:"request"`
	Action  map[string]json.RawMessage `json:"action"`
}

func (s *Server) Act(input ActionRequest) (any, error) {
	s.actionsMu.Lock()
	defer s.actionsMu.Unlock()
	name, sub, err := decodeRequest(input.Request)
	if err != nil {
		return nil, err
	}
	c, state, _, err := s.selection(sub)
	if err != nil {
		return nil, err
	}
	if len(input.Action) != 1 {
		return nil, fmt.Errorf("action requires one component")
	}
	raw, ok := input.Action[name]
	if !ok {
		return nil, fmt.Errorf("action component mismatch")
	}
	var action map[string]json.RawMessage
	if err = json.Unmarshal(raw, &action); err != nil {
		return nil, err
	}
	if len(action) != 1 {
		return nil, fmt.Errorf("action requires one operation")
	}
	command := service.Command{ID: input.ID, Proposal: c.Material.Proposal, Candidate: c.Material.ID}
	var payload struct {
		Path       string  `json:"path"`
		Hunk       string  `json:"hunk"`
		Directory  string  `json:"directory"`
		Index      int     `json:"index"`
		Decision   string  `json:"decision"`
		Timestamp  string  `json:"timestamp"`
		Candidate  string  `json:"candidate"`
		Comment    string  `json:"comment"`
		Text       string  `json:"text"`
		Permission *string `json:"permission"`
		Keep       bool    `json:"keep_signatures"`
		Opened     bool    `json:"diff_was_opened"`
	}
	operation := ""
	for op, body := range action {
		operation = op
		if err = json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
	}
	target := ""
	if payload.Hunk != "" {
		for _, h := range c.Material.Hunks {
			if h.Path == payload.Path && (header(h) == payload.Hunk || h.Header == payload.Hunk) {
				target = h.ID
				break
			}
		}
		if target == "" {
			return nil, fmt.Errorf("unknown hunk")
		}
	}
	switch operation {
	case "OpenDiff":
		command.Action = "open_diff"
	case "Sign":
		command.Action = "sign"
		command.Target = target
	case "SignAllUnder":
		command.Action = "sign_under"
		command.Target = payload.Directory
	case "Rule":
		if payload.Index < 1 || payload.Index > len(c.Material.Doubts) {
			return nil, fmt.Errorf("unknown doubt")
		}
		command.Action = "rule"
		command.Target = c.Material.Doubts[payload.Index-1].ID
		command.Value = payload.Decision
	case "Comment":
		command.Action = "comment"
		command.Target = target
		command.Text = payload.Text
	case "AppendEntry":
		command.Action = "comment"
		command.Text = payload.Comment
		command.KeepSignatures = payload.Keep
		if payload.Candidate != "" {
			command.Candidate = payload.Candidate
		}
		if payload.Permission != nil {
			command.Action = "verdict"
			command.Value = *payload.Permission
		}
		if payload.Opened && !state.Opened {
			if _, err = s.Service.Submit(service.Command{ID: input.ID + "-open", Proposal: command.Proposal, Candidate: command.Candidate, Action: "open_diff"}); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported %s action %s", name, operation)
	}
	var signWrite object
	if operation == "Sign" {
		lines := 0
		for _, h := range c.Material.Hunks {
			if h.ID == target {
				lines = changed(h)
				break
			}
		}
		signWrite = object{"path": payload.Path, "hunk": payload.Hunk, "changedLines": lines, "requiredWaitMilliseconds": lines * 10, "outcome": "signed"}
		if _, signed := state.Signatures[target]; signed {
			signWrite["outcome"] = "already_signed"
		} else {
			var previous time.Time
			for _, signature := range state.Signatures {
				if signature.CarriedFrom != "" {
					continue
				}
				at, err := time.Parse(time.RFC3339Nano, signature.At)
				if err != nil {
					return nil, err
				}
				if at.After(previous) {
					previous = at
				}
			}
			if !previous.IsZero() {
				remaining := int64(lines*10) - time.Since(previous).Milliseconds()
				if remaining > 0 {
					signWrite["outcome"] = "refused_as_too_fast"
					signWrite["remainingMs"] = remaining
					signWrite["message"] = "You sign too fast. Are you serious?"
					view, err := s.View(input.Request)
					if err != nil {
						return nil, err
					}
					view.(object)["write"] = signWrite
					return view, nil
				}
			}
		}
	}

	receipt, err := s.Service.Submit(command)
	if err != nil {
		return nil, err
	}
	view, err := s.View(input.Request)
	if err != nil {
		return nil, err
	}
	// The component acknowledges its own timestamp; the journal keeps server time.
	// This response-only field never changes stored chronology or signature identity.
	if operation == "AppendEntry" {
		v := view.(object)
		entries := v["entries"].([]object)
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			e := entry["entry"].(object)
			if e["candidate"] == command.Candidate && e["body"] == command.Text {
				e["timestamp"] = payload.Timestamp
				break
			}
		}
	}
	if operation == "Sign" || operation == "SignAllUnder" {
		v := view.(object)
		v["sign_presses"] = receipt.Sequence
		v["write"] = signWrite
	}
	return view, nil
}

// HunkText is the pinned context for a user-requested Peek message.
func (s *Server) HunkText(sub Subject, p, hunk string) (string, error) {
	c, _, _, err := s.selection(sub)
	if err != nil {
		return "", err
	}
	for _, h := range c.Material.Hunks {
		if h.Path == p && (header(h) == hunk || h.Header == hunk) {
			return strings.Join([]string{p, h.Header, h.Patch}, "\n"), nil
		}
	}
	return "", fmt.Errorf("unknown hunk")
}
