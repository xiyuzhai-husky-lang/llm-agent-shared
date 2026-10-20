package plan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"proposal-platform/httpjson"
	"proposal-workflow/internal/material"
)

type PageComment struct {
	ID        string `json:"id"`
	Anchor    string `json:"anchor"`
	Quote     string `json:"quote"`
	Text      string `json:"text"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
}
type PageInput struct {
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}
type pageComments struct {
	Format   int           `json:"format"`
	Comments []PageComment `json:"comments"`
}
type pageInputs struct {
	Format int                  `json:"format"`
	Inputs map[string]PageInput `json:"inputs"`
}

func readPage(path string, value any) error {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return material.Decode(string(raw), value)
}
func savePage(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".page-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
func (s *Service) Handler(ctx context.Context, web string) (http.Handler, error) {
	command := exec.CommandContext(ctx, "git", "-C", s.Config.Main, "config", "user.name")
	raw, err := command.Output()
	if err != nil {
		return nil, err
	}
	signer := strings.TrimSpace(string(raw))
	if signer == "" {
		return nil, fmt.Errorf("git user.name is empty")
	}
	mux := http.NewServeMux()
	var pages sync.Mutex
	fail := func(w http.ResponseWriter, err error) {
		httpjson.Write(w, 400, map[string]string{"error": err.Error()})
	}
	mux.HandleFunc("POST /api/plans/start", func(w http.ResponseWriter, r *http.Request) {
		var request Start
		if err := httpjson.Decode(w, r, &request); err != nil {
			fail(w, err)
			return
		}
		if err := s.Start(ctx, request); err != nil {
			fail(w, err)
			return
		}
		httpjson.Write(w, 200, map[string]string{"started": request.Instance})
	})
	mux.HandleFunc("GET /api/plans/{instance}/status", func(w http.ResponseWriter, r *http.Request) {
		state, err := s.Status(r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		httpjson.Write(w, 200, state)
	})
	mux.HandleFunc("GET /api/plans/{instance}/review", func(w http.ResponseWriter, r *http.Request) {
		view, err := s.Review(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		httpjson.Write(w, 200, view)
	})
	for _, action := range []string{"answer", "verdict", "reset"} {
		mux.HandleFunc("POST /api/plans/{instance}/"+action, func(w http.ResponseWriter, r *http.Request) {
			var request struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			}
			if action != "reset" {
				if err := httpjson.Decode(w, r, &request); err != nil {
					fail(w, err)
					return
				}
			}
			instance := r.PathValue("instance")
			if err := s.Action(ctx, instance, action, request.Kind, request.Text); err != nil {
				fail(w, err)
				return
			}
			httpjson.Write(w, 200, map[string]string{action: instance})
		})
	}
	mux.HandleFunc("GET /plans/{instance}", func(w http.ResponseWriter, r *http.Request) {
		if !validInstance(r.PathValue("instance")) {
			fail(w, fmt.Errorf("invalid instance"))
			return
		}
		http.ServeFile(w, r, filepath.Join(web, "index.html"))
	})
	mux.HandleFunc("GET /plans/{instance}/plan/{$}", func(w http.ResponseWriter, r *http.Request) {
		pages.Lock()
		defer pages.Unlock()
		directory, err := s.page(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		http.ServeFile(w, r, filepath.Join(directory, "index.html"))
	})
	mux.HandleFunc("/plans/{instance}/plan/api/comments", func(w http.ResponseWriter, r *http.Request) {
		pages.Lock()
		defer pages.Unlock()
		directory, err := s.page(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		path := filepath.Join(directory, "comments.json")
		store := pageComments{1, []PageComment{}}
		if err := readPage(path, &store); err != nil {
			fail(w, err)
			return
		}
		if store.Format != 1 {
			fail(w, fmt.Errorf("invalid comments format"))
			return
		}
		switch r.Method {
		case "GET":
			httpjson.Write(w, 200, map[string]any{"comments": store.Comments})
		case "POST":
			var request struct {
				Anchor string `json:"anchor"`
				Quote  string `json:"quote"`
				Text   string `json:"text"`
				Author string `json:"author"`
			}
			if err := httpjson.Decode(w, r, &request); err != nil {
				fail(w, err)
				return
			}
			anchor, text := strings.TrimSpace(request.Anchor), strings.TrimSpace(request.Text)
			if anchor == "" || len(anchor) > 200 || text == "" || len(text) > 20000 {
				fail(w, fmt.Errorf("anchor and text are required and must fit their limits"))
				return
			}
			author := strings.TrimSpace(request.Author)
			if author == "" {
				author = signer
			}
			if len(author) > 80 {
				fail(w, fmt.Errorf("author too long"))
				return
			}
			quote := []rune(strings.Join(strings.Fields(request.Quote), " "))
			if len(quote) > 2000 {
				quote = quote[:2000]
			}
			var id [6]byte
			if _, err := rand.Read(id[:]); err != nil {
				fail(w, err)
				return
			}
			comment := PageComment{hex.EncodeToString(id[:]), anchor, string(quote), text, author, time.Now().UTC().Format(time.RFC3339Nano)}
			store.Comments = append(store.Comments, comment)
			if err := savePage(path, store); err != nil {
				fail(w, err)
				return
			}
			httpjson.Write(w, 201, comment)
		default:
			w.WriteHeader(405)
		}
	})
	mux.HandleFunc("DELETE /plans/{instance}/plan/api/comments/{id}", func(w http.ResponseWriter, r *http.Request) {
		pages.Lock()
		defer pages.Unlock()
		directory, err := s.page(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		path := filepath.Join(directory, "comments.json")
		store := pageComments{1, []PageComment{}}
		if err := readPage(path, &store); err != nil {
			fail(w, err)
			return
		}
		if store.Format != 1 {
			fail(w, fmt.Errorf("invalid comments format"))
			return
		}
		found := false
		for i, c := range store.Comments {
			if c.ID == r.PathValue("id") {
				store.Comments = append(store.Comments[:i], store.Comments[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			fail(w, fmt.Errorf("no such comment"))
			return
		}
		if err := savePage(path, store); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /plans/{instance}/plan/api/inputs", func(w http.ResponseWriter, r *http.Request) {
		pages.Lock()
		defer pages.Unlock()
		directory, err := s.page(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		store := pageInputs{1, map[string]PageInput{}}
		if err := readPage(filepath.Join(directory, "inputs.json"), &store); err != nil {
			fail(w, err)
			return
		}
		if store.Format != 1 {
			fail(w, fmt.Errorf("invalid inputs format"))
			return
		}
		httpjson.Write(w, 200, map[string]any{"inputs": store.Inputs})
	})
	mux.HandleFunc("PUT /plans/{instance}/plan/api/inputs/{name}", func(w http.ResponseWriter, r *http.Request) {
		pages.Lock()
		defer pages.Unlock()
		name := r.PathValue("name")
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(name) {
			fail(w, fmt.Errorf("invalid input name"))
			return
		}
		var request struct {
			Value string `json:"value"`
		}
		if err := httpjson.Decode(w, r, &request); err != nil {
			fail(w, err)
			return
		}
		if len(request.Value) > 20000 {
			fail(w, fmt.Errorf("value too long"))
			return
		}
		directory, err := s.page(r.Context(), r.PathValue("instance"))
		if err != nil {
			fail(w, err)
			return
		}
		path := filepath.Join(directory, "inputs.json")
		store := pageInputs{1, map[string]PageInput{}}
		if err := readPage(path, &store); err != nil {
			fail(w, err)
			return
		}
		if store.Format != 1 {
			fail(w, fmt.Errorf("invalid inputs format"))
			return
		}
		value := PageInput{request.Value, time.Now().UTC().Format(time.RFC3339Nano)}
		store.Inputs[name] = value
		if err := savePage(path, store); err != nil {
			fail(w, err)
			return
		}
		httpjson.Write(w, 200, value)
	})
	return mux, nil
}
