package reviewer

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/httpjson"
)

func (s *Server) registerProjectPage(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/proposal-events", s.projectEvents)
	mux.HandleFunc("GET /api/proposal-identity", func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, 200, object{"signer": s.Service.ReviewSnapshot().Signer})
	})
	for _, route := range []string{"proposal-snapshot", "proposal-blob", "proposal-diff-file", "proposal-review", "proposal-signatures", "proposal-diff-open", "proposal-candidate"} {
		mux.HandleFunc("/api/"+route, s.projectRequest)
	}
}
func (s *Server) projectRequest(w http.ResponseWriter, r *http.Request) {
	if err := s.handleProjectRequest(w, r); err != nil {
		httpjson.Error(w, err)
	}
}
func (s *Server) handleProjectRequest(w http.ResponseWriter, r *http.Request) error {
	if err := s.Prepare(r.Context()); err != nil {
		return err
	}
	query := r.URL.Query()
	revision := query.Get("revision")
	if revision == "" {
		revision = query.Get("candidate")
	}
	c, state, snap, err := s.projectSelection(query.Get("proposal"), revision)
	if err != nil {
		return err
	}
	route := strings.TrimPrefix(r.URL.Path, "/api/")
	if r.Method == http.MethodGet {
		switch route {
		case "proposal-snapshot":
			// These pages display committed PDFs. Missing promised assets fail by name.
			for _, name := range []string{"problem", "diagnosis", "design", "implementationPlan"} {
				if c.Material.Kind == "complete" && c.Page.PDFs[name] == nil {
					return fmt.Errorf("candidate %s has no committed %s PDF", c.Material.ID, name)
				}
			}
			httpjson.Write(w, 200, s.projectSnapshot(c, state, snap))
		case "proposal-candidate":
			allowed := false
			if p := query.Get("path"); p != "" {
				_, allowed = c.Page.Blobs[p]
			} else {
				allowed = true
			}
			httpjson.Write(w, 200, object{"available": allowed, "folder": c.Folder, "candidate": c.Material.ID})
		case "proposal-blob":
			p := query.Get("path")
			data, ok := c.Page.Blobs[p]
			if p == projectPath(c.Folder, "diff.patch") {
				data = ""
				ok = true
			} // File bodies are loaded on demand from the cached diff.
			if !ok {
				return fmt.Errorf("path is not displayed by the delivered candidate: %s", p)
			}
			media := "text/plain; charset=utf-8"
			if strings.HasSuffix(p, ".pdf") {
				media = "application/pdf"
			} else if strings.HasSuffix(p, ".json") {
				media = "application/json"
			}
			w.Header().Set("Content-Type", media)
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
			fmt.Fprint(w, data)
		case "proposal-diff-file":
			file, ok := c.ProjectDiff.Files[query.Get("path")]
			if !ok {
				return fmt.Errorf("file is not in this candidate")
			}
			httpjson.Write(w, 200, file)
		case "proposal-review":
			httpjson.Write(w, 200, s.projectReview(c, state, s.Service.ReviewSnapshot(c.Material.Proposal)))
		case "proposal-signatures":
			httpjson.Write(w, 200, c.projectSignatures(state))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return nil
	}
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	var input struct {
		ID         string  `json:"id"`
		Comment    string  `json:"comment"`
		Permission *string `json:"permission"`
		Keep       bool    `json:"keepSignatures"`
		Path       string  `json:"path"`
		Hunk       string  `json:"hunk"`
	}
	if err := httpjson.Decode(w, r, &input); err != nil {
		return err
	}
	if input.ID == "" {
		return fmt.Errorf("command id required")
	}
	command := service.Command{ID: input.ID, Proposal: c.Material.Proposal, Candidate: c.Material.ID}
	switch route {
	case "proposal-diff-open":
		command.Action = "open_diff"
		if _, err := s.Service.Submit(command); err != nil {
			return err
		}
		httpjson.Write(w, 200, object{})
	case "proposal-review":
		command.Action = "comment"
		command.Text = input.Comment
		command.KeepSignatures = input.Keep
		if input.Permission != nil {
			command.Action = "verdict"
			command.Value = *input.Permission
		} else if match := regexp.MustCompile(`^Doubt (\d+) (sustained|dismissed|followup|defer|revoked):`).FindStringSubmatch(input.Comment); match != nil {
			index, _ := strconv.Atoi(match[1])
			if index < 1 || index > len(c.Material.Doubts) {
				return fmt.Errorf("unknown Doubt")
			}
			command.Action = "rule"
			command.Target = c.Material.Doubts[index-1].ID
			command.Value = match[2]
		}
		if _, err := s.Service.Submit(command); err != nil {
			return err
		}
		_, updated, current, err := s.selection(c.Subject)
		if err != nil {
			return err
		}
		httpjson.Write(w, 200, s.projectReview(c, updated, current))
	case "proposal-signatures":
		target := ""
		for _, h := range c.Material.Hunks {
			if h.Path == input.Path && projectHunkKey(h) == input.Hunk {
				target = header(h)
				break
			}
		}
		if target == "" {
			return fmt.Errorf("unknown hunk")
		}
		view, err := s.Act(actionFor("DiffTab", c, input.ID, "Sign", object{"path": input.Path, "hunk": target}))
		if err != nil {
			return err
		}
		if write, ok := view.(object)["write"].(object); ok && write["outcome"] == "refused_as_too_fast" {
			httpjson.Write(w, 429, object{"error": write["message"], "remainingMs": write["remainingMs"]})
			return nil
		}
		_, updated, _, err := s.selection(c.Subject)
		if err != nil {
			return err
		}
		response := c.projectSignatures(updated)
		for _, row := range c.ProjectDiff.View["presentation"].(object)["navigation"].([]object) {
			if row["path"] == input.Path && row["hunk"] == input.Hunk {
				response["next"] = row["next"]
				break
			}
		}
		httpjson.Write(w, 200, response)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	return nil
}
func projectVersion(decision, workflow api.Frame) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s:%d/%s:%d", decision.Epoch, decision.Cursor, workflow.Epoch, workflow.Cursor)
	return h.Sum64() & ((1 << 52) - 1)
}
func (s *Server) projectEvents(w http.ResponseWriter, r *http.Request) {
	decision := s.Service.Views.Position()
	var workflow api.Frame
	if s.Workflow != nil {
		workflow = s.Workflow.changes.Position()
	}
	version := projectVersion(decision, workflow)
	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	if r.URL.Query().Has("since") && since == version {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		changed := make(chan struct{}, 2)
		go func() {
			s.Service.Views.Wait(ctx, decision.Epoch, decision.Cursor, []string{"proposal/"}, 20*time.Second)
			changed <- struct{}{}
		}()
		if s.Workflow != nil {
			go func() {
				s.Workflow.changes.Wait(ctx, workflow.Epoch, workflow.Cursor, []string{"workflow"}, 20*time.Second)
				changed <- struct{}{}
			}()
		}
		select {
		case <-changed:
		case <-ctx.Done():
		}
		cancel()
		decision = s.Service.Views.Position()
		if s.Workflow != nil {
			workflow = s.Workflow.changes.Position()
		}
		version = projectVersion(decision, workflow)
	}
	httpjson.Write(w, 200, object{"version": version})
}
