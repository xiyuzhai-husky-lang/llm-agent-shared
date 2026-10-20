package httpserver_test

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"proposal-platform/journal"
	"proposal-workflow/internal/httpserver"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

// Changing a server-derived slot/date default cannot invalidate a Start receipt.
func TestStartRetryUsesOriginalRequest(t *testing.T) {
	root := t.TempDir()
	j, err := journal.Open(filepath.Join(root, "workflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	app, err := service.Open(j, map[string]workflow.Policy{"D": {Rank: "D"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []ledger.Action{{Kind: "open", Batch: "b"}, {Kind: "add", Batch: "b", UID: "p"}, {Kind: "approve", Batch: "b", Source: "test"}} {
		if _, err = app.Submit(service.Command{ID: a.Kind, Action: "ledger", Ledger: &a}); err != nil {
			t.Fatal(err)
		}
	}
	config := &project.Config{Project: "projects/example", State: filepath.Join(root, "first")}
	handler := (&httpserver.Server{Project: config, Service: app}).Handler()
	request := []byte(`{"id":"start","action":"start","uid":"p","slot":"1","rank":"D"}`)
	submit := func(body []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/commands", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	first := submit(request)
	if first.Code != 200 {
		t.Fatal(first.Body.String())
	}
	config.State = filepath.Join(root, "changed-default")
	retry := submit(request)
	if retry.Code != 200 || retry.Body.String() != first.Body.String() {
		t.Fatalf("retry differs: %d %s", retry.Code, retry.Body.String())
	}
	changed := submit(bytes.Replace(request, []byte(`"rank":"D"`), []byte(`"rank":"D","assignment":"different"`), 1))
	if changed.Code != 409 {
		t.Fatal("conflicting input accepted")
	}
}
