package httpserver

import (
	"net/http"
	"proposal-decision/internal/reviewer"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/httpjson"
)

// Server owns HTTP translation and serving the decision browser.
type Server struct {
	Reviewer *reviewer.Server
	Service  *service.Service
	WebRoot  string
	Peek     http.Handler
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	if s.Reviewer != nil {
		s.Reviewer.Register(mux)
	}
	if s.Peek != nil {
		mux.Handle("/api/peek", s.Peek)
	}
	mux.HandleFunc("GET /api/peek-context", func(w http.ResponseWriter, r *http.Request) {
		material, ok := s.Service.PeekMaterial(r.URL.Query().Get("proposal"), r.URL.Query().Get("candidate"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		httpjson.Write(w, 200, material)
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpjson.Write(w, 200, map[string]any{"program": "proposal-decision", "cost": s.Service.Cost()})
	})
	mux.HandleFunc("POST /api/deliveries", func(w http.ResponseWriter, r *http.Request) {
		var input api.Delivery
		if err := httpjson.Decode(w, r, &input); err != nil {
			httpjson.Error(w, err)
			return
		}
		receipt, err := s.Service.Deliver(r.Context(), input)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		if s.Reviewer != nil {
			if err := s.Reviewer.Prepare(r.Context()); err != nil {
				httpjson.Error(w, err)
				return
			}
		}
		httpjson.Write(w, 200, receipt)
	})
	mux.HandleFunc("POST /api/commands", func(w http.ResponseWriter, r *http.Request) {
		var input service.Command
		if err := httpjson.Decode(w, r, &input); err != nil {
			httpjson.Error(w, err)
			return
		}
		receipt, err := s.Service.Submit(input)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, receipt)
	})
	mux.HandleFunc("GET /api/decisions", func(w http.ResponseWriter, r *http.Request) {
		after, err := httpjson.Cursor(r)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		feed, err := s.Service.Feed(r.Context(), r.URL.Query().Get("journal"), after, httpjson.Wait(r))
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		httpjson.Write(w, 200, feed)
	})
	mux.HandleFunc("GET /api/views", func(w http.ResponseWriter, r *http.Request) {
		after, err := httpjson.Cursor(r)
		if err != nil {
			httpjson.Error(w, err)
			return
		}
		prefixes := r.URL.Query()["prefix"]
		if len(prefixes) == 0 {
			prefixes = []string{"proposal/"}
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
