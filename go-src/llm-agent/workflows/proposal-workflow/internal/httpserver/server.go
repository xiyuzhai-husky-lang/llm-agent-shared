package httpserver

import (
	"context"
	"net/http"
	"path/filepath"
	"proposal-platform/httpjson"
	"proposal-platform/journal"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/service"
	"time"
)

// Server owns strict workflow HTTP translation and browser serving.
type Server struct {
	Tick        func(context.Context, int) error
	Project     *project.Config
	Service     *service.Service
	WebRoot     string
	DecisionURL string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/tick", func(w http.ResponseWriter, r *http.Request) {
		if s.Tick == nil {
			http.Error(w, "tick is available only in script mode", 400)
			return
		}
		var input struct {
			Ticks int `json:"ticks"`
		}
		if err := httpjson.Decode(w, r, &input); err != nil {
			httpjson.Error(w, err)
			return
		}
		if input.Ticks < 1 || input.Ticks > 10000 {
			http.Error(w, "invalid tick count", 400)
			return
		}
		if err := s.Tick(r.Context(), input.Ticks); err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, map[string]int{"ticks": input.Ticks})
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, 200, map[string]string{"decision_url": s.DecisionURL})
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		identity, cursor := s.Service.Checkpoint()
		health := map[string]any{"program": "proposal-workflow", "journal": identity, "cursor": cursor, "views": s.Service.ViewCount()}
		if s.Project != nil {
			health["project"] = s.Project.Project
			health["main"] = s.Project.Main
			health["target"] = s.Project.TargetBranch
		}
		httpjson.Write(w, 200, health)
	})
	mux.HandleFunc("POST /api/commands", func(w http.ResponseWriter, r *http.Request) {
		var c service.Command
		if err := httpjson.Decode(w, r, &c); err != nil {
			httpjson.Error(w, err)
			return
		}
		digest, err := journal.Digest(c)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		c.RequestDigest = digest
		if c.Action == "start" && s.Project != nil {
			if c.Directory == "" {
				c.Directory = filepath.Join(s.Project.State, "slots", c.Slot)
			}
			var err error
			c.Directory, err = canonicalDirectory(c.Directory)
			if err != nil {
				httpjson.Error(w, err)
				return
			}
			if c.Folder == "" {
				c.Folder = filepath.ToSlash(filepath.Join(s.Project.ProposalsPath, time.Now().Format("2006/01/02"), c.UID))
			}
		}
		receipt, err := s.Service.Submit(c)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, receipt)
	})
	mux.HandleFunc("GET /api/views", func(w http.ResponseWriter, r *http.Request) {
		after, err := httpjson.Cursor(r)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		prefixes := r.URL.Query()["prefix"]
		if len(prefixes) == 0 {
			prefixes = []string{"proposal/", "ledger", "feed"}
		}
		frame := s.Service.Views.Wait(r.Context(), r.URL.Query().Get("epoch"), after, prefixes, httpjson.Wait(r))
		httpjson.Write(w, 200, frame)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	if s.WebRoot != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.WebRoot)))
	}
	return mux
}
