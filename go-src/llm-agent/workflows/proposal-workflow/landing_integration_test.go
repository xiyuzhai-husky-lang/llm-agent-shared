package workflow_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"proposal-platform/api"
	"proposal-platform/journal"
	"proposal-workflow/internal/decisionclient"
	"proposal-workflow/internal/execution"
	"proposal-workflow/internal/httpserver"
	"proposal-workflow/internal/landing"
	"proposal-workflow/internal/ledger"
	"proposal-workflow/internal/material"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/service"
	"proposal-workflow/internal/workflow"
)

// TestAcceptedLandingIntegration exercises the production decision HTTP server,
// workflow HTTP server, feed subscriber, coordinator, and Lander. Role execution,
// project gates, and Outcome measurements are explicitly identified fixtures.
// Every Git write stays in a new independent /private/tmp repository and its own
// worktree and bare remote. Evidence is deliberately retained, even on failure.
// Run with PROPOSAL_LANDING_INTEGRATION=1 go test -run '^TestAcceptedLandingIntegration$' -count=1 -v .
func TestAcceptedLandingIntegration(t *testing.T) {
	if os.Getenv("PROPOSAL_LANDING_INTEGRATION") != "1" {
		t.Skip("opt-in: creates and retains an isolated scratch Git repository")
	}
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		if os.Getenv(key) != "" {
			t.Fatalf("unset %s before running isolated Git integration", key)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	root, err := os.MkdirTemp("/private/tmp", "proposal-landing-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained scratch evidence: %s", root)
	main, slot, remote := filepath.Join(root, "main"), filepath.Join(root, "slot"), filepath.Join(root, "origin.git")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	git := func(directory string, args ...string) string {
		t.Helper()
		rel, err := filepath.Rel(root, directory)
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatalf("Git directory escaped scratch: %s", directory)
		}
		command := exec.CommandContext(ctx, "git", args...)
		command.Dir = directory
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q in %s: %v\n%s", args, directory, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	git(root, "init", "--bare", "--initial-branch=main", remote)
	git(root, "init", "--initial-branch=main", main)
	git(main, "config", "user.name", "Scratch Reviewer")
	git(main, "config", "user.email", "scratch-reviewer@example.invalid")
	git(main, "config", "commit.gpgsign", "false")
	git(main, "remote", "add", "origin", remote)
	const uid = "batch-26-1"
	const folder = "projects/fixture/proposals/batch-26-1"
	write(filepath.Join(main, folder, "revision.json"), "{\"fixture\":true}\n", 0600)
	write(filepath.Join(main, "address_of.c"), "int address_of(int x) { return x; }\n", 0600)
	write(filepath.Join(main, "dereference.c"), "int dereference(int *p) { return 0; }\n", 0600)
	write(filepath.Join(main, "Makefile"), ".PHONY: royal-guard\nroyal-guard:\n\t@test -f address_of.c\n\t@echo 'FIXTURE royal-guard passed'\n", 0600)
	write(filepath.Join(main, "projects/fixture/tools/check-uids.sh"), "#!/bin/bash\nset -eu\ntest \"$PWD\" = \"$PROPOSAL_MAIN\"\ntest -f projects/fixture/proposals/batch-26-1/revision.json\nprintf 'FIXTURE UID check passed\\n'\n", 0700)
	write(filepath.Join(main, "projects/fixture/outcome_fragments.txt"), "fixture-landed-tree\n", 0600)
	write(filepath.Join(main, "projects/common/outcome/fragments/fixture-landed-tree/fragment.toml"), "title = \"Fixture landed tree\"\nreport = \"statistics.json\"\n", 0600)
	write(filepath.Join(main, "projects/common/outcome/run.sh"), `#!/bin/bash
set -eu
test -d "$1"
test -n "$OUTCOME_BASE"
test -n "$OUTCOME_LOCK"
mkdir -p "$2"
commit=$(git rev-parse HEAD)
tree=$(git rev-parse 'HEAD^{tree}')
printf '{"format":1,"commit":"%s","reports":[{"fragment":"fixture-landed-tree","file":"statistics.json","elapsed_seconds":0}]}\n' "$commit" > "$2/run.json"
printf '{"commit":"%s","tree":"%s","base":"%s","status":"passed","fixture":true}\n' "$commit" "$tree" "$OUTCOME_BASE" > "$2/fixture.json"
printf '{"format":1,"title":"Fixture landed tree","status":"passed","commit":"%s","counts":[],"table":{"columns":["tree","base"],"rows":[["%s","%s"]]}}\n' "$commit" "$tree" "$OUTCOME_BASE" > "$2/statistics.json"
printf 'FIXTURE Outcome measured commit %s tree %s\n' "$commit" "$tree"
`, 0700)
	git(main, "add", "--all")
	git(main, "commit", "-m", "Fixture: isolated accepted-landing base")
	base := git(main, "rev-parse", "HEAD")
	git(main, "push", "--set-upstream", "origin", "main")
	git(main, "worktree", "add", "-b", "proposal/"+uid, slot, "main")
	write(filepath.Join(slot, "address_of.c"), "int address_of(int x) { int *p = &x; return *p; }\n", 0600)
	write(filepath.Join(slot, "dereference.c"), "int dereference(int *p) { return *p; }\n", 0600)
	git(slot, "add", "--all")
	git(slot, "commit", "-m", "Fixture: address-of and dereference candidate")
	candidateID := git(slot, "rev-parse", "HEAD")
	if got := git(main, "worktree", "list", "--porcelain"); !strings.Contains(got, "worktree "+slot) || strings.Count(got, "worktree ") != 2 {
		t.Fatalf("unexpected scratch worktree registration: %s", got)
	}
	if got := git(main, "remote", "get-url", "origin"); got != remote {
		t.Fatalf("unexpected scratch remote: %s", got)
	}

	decisionURL := startScratchDecision(t, ctx, root, main)
	journalPath := filepath.Join(root, "workflow.sqlite")
	j, err := journal.Open(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	app, err := service.Open(j, map[string]workflow.Policy{"fixture": {Rank: "fixture", Model: "fixture", Effort: "fixture", DeadlineSeconds: 120}})
	if err != nil {
		t.Fatal(err)
	}
	config := &project.Config{
		Repository: main, Main: main, Project: "projects/fixture", State: filepath.Join(root, "workflow-state"),
		TargetBranch: "main", BranchPrefix: "proposal/", LandingOutcome: "measure-landed-tree",
		LandingMainChanges: "merge-check", LandingUIDCheck: "environment", LandingReviewHooks: "run", LandingMergeHooks: "run",
		LandingTrailerLiterals: []string{"Committed-By: scratch accepted-landing integration test"},
	}
	candidate := api.Candidate{
		Proposal: uid, ID: candidateID, Title: "FIXTURE C pointers: address-of and dereference", Kind: "complete", Base: base,
		Source:    &api.Source{Repository: slot, Base: base, Commit: candidateID, MustSign: []string{"address_of.c", "dereference.c"}},
		Doubts:    []api.Doubt{{ID: "fixture-doubt", Source: "fixture-reviewer", Text: "FIXTURE: inspect address-of and dereference", Path: "address_of.c", Header: "@@ -1 +1 @@", Worker: "disagreed", Reason: "FIXTURE response"}},
		Documents: []api.Document{{ID: "fixture-notice", Kind: "role", Title: "Fixture boundary", Text: "Roles, gates, and Outcome are fixtures; delivery, decisions, feed, coordinator, Lander, and Git are production."}},
	}
	runner := &scratchLandingRunner{candidate: candidate, lander: &landing.Lander{Config: config, Repository: repository.Repository{Config: config}}}
	coordinator := &execution.Coordinator{Service: app, Runner: runner, Decision: &decisionclient.Client{URL: decisionURL, HTTP: &http.Client{Timeout: 25 * time.Second}}}
	server := httptest.NewServer((&httpserver.Server{Project: config, Service: app, DecisionURL: decisionURL}).Handler())
	defer server.Close()
	runCtx, stop := context.WithCancel(ctx)
	var group sync.WaitGroup
	group.Add(2)
	go func() { defer group.Done(); coordinator.Run(runCtx) }()
	go func() { defer group.Done(); coordinator.Subscribe(runCtx) }()
	defer func() { stop(); group.Wait() }()

	post := func(baseURL string, input any) api.Receipt {
		t.Helper()
		var receipt api.Receipt
		scratchRequest(t, ctx, "POST", baseURL+"/api/commands", input, http.StatusOK, &receipt)
		return receipt
	}
	for i, action := range []ledger.Action{
		{Kind: "open", Batch: "fixture-batch-26", Title: "FIXTURE batch 26"},
		{Kind: "add", Batch: "fixture-batch-26", UID: uid, Title: candidate.Title},
		{Kind: "approve", Batch: "fixture-batch-26", Source: "integration-test fixture authorization"},
	} {
		post(server.URL, service.Command{ID: fmt.Sprintf("fixture-ledger-%d", i), Action: "ledger", Ledger: &action})
	}
	post(server.URL, service.Command{ID: "fixture-start", Action: "start", UID: uid, Slot: "fixture-slot", Directory: slot, Folder: folder, Base: base, Rank: "fixture", Assignment: "Explicit fixture for accepted-landing integration"})
	waitScratchProposal(t, ctx, server.URL, uid, "present")
	var frame api.Frame
	scratchRequest(t, ctx, "GET", decisionURL+"/api/views?prefix=candidate/"+uid+"/"+candidateID, nil, http.StatusOK, &frame)
	var delivered struct {
		Hunks []string `json:"hunks"`
	}
	if err := json.Unmarshal(frame.Puts["candidate/"+uid+"/"+candidateID], &delivered); err != nil || len(delivered.Hunks) != 2 {
		t.Fatalf("expected two Git-derived required hunks: %+v (%v)", delivered, err)
	}
	highlightedPaths := []string{}
	for _, id := range delivered.Hunks {
		key := "hunk/" + uid + "/" + candidateID + "/" + id
		scratchRequest(t, ctx, "GET", decisionURL+"/api/views?prefix="+key, nil, http.StatusOK, &frame)
		var view struct{ Hunk api.Hunk }
		if err := json.Unmarshal(frame.Puts[key], &view); err != nil {
			t.Fatal(err)
		}
		if view.Hunk.Highlight == nil || view.Hunk.Highlight.Language != "c" || !strings.Contains(view.Hunk.Highlight.Before, "<span") || !strings.Contains(view.Hunk.Highlight.After, "<span") {
			t.Fatalf("Git-delivered C hunk lacks server-rendered syntax: %+v", view.Hunk)
		}
		highlightedPaths = append(highlightedPaths, view.Hunk.Path)
	}
	decisionCommand := func(id, action, target, value string) map[string]string {
		return map[string]string{"id": id, "proposal": uid, "candidate": candidateID, "action": action, "target": target, "value": value}
	}
	post(decisionURL, decisionCommand("fixture-open", "open_diff", "", ""))
	first := post(decisionURL, decisionCommand("fixture-sign-first", "sign", delivered.Hunks[0], ""))
	if first.Next != "hunk/"+uid+"/"+candidateID+"/"+delivered.Hunks[1] {
		t.Fatalf("Sign must jump past its still-unruled Doubt, got next=%s", first.Next)
	}
	post(decisionURL, decisionCommand("fixture-sign-second", "sign", delivered.Hunks[1], ""))
	scratchRequest(t, ctx, "POST", decisionURL+"/api/commands", decisionCommand("fixture-premature-accept", "verdict", "", "accept"), http.StatusConflict, nil)
	post(decisionURL, decisionCommand("fixture-rule", "rule", "fixture-doubt", "dismissed"))
	acceptedReceipt := post(decisionURL, decisionCommand("fixture-accept", "verdict", "", "accept"))
	accepted := waitScratchProposal(t, ctx, server.URL, uid, "accepted")
	if accepted.Verdict == nil || accepted.Verdict.Signer != "Scratch Reviewer" || accepted.Verdict.Candidate != candidateID {
		t.Fatalf("workflow did not receive fixture Accept through the real feed: %+v", accepted.Verdict)
	}
	var feed api.Feed
	scratchRequest(t, ctx, "GET", decisionURL+"/api/decisions?after=0", nil, http.StatusOK, &feed)
	if len(feed.Decisions) != 4 || feed.Decisions[3].Sequence != acceptedReceipt.Sequence || feed.Decisions[3].Value != "accept" {
		t.Fatalf("unexpected Sign/Sign/Rule/Accept decision feed: %+v", feed)
	}
	post(server.URL, service.Command{ID: "fixture-land", Action: "land", UID: uid})
	landed := waitScratchProposal(t, ctx, server.URL, uid, "landed")

	var record landing.Record
	readScratchJSON(t, filepath.Join(config.State, "landings", uid, "landing.json"), &record)
	if record.Step != "complete" || record.Candidate != candidateID || record.TargetBefore != base || record.Review == "" || record.Merge == "" {
		t.Fatalf("incomplete landing record: %+v", record)
	}
	if got := git(main, "show", "-s", "--format=%P", record.Merge); got != base+" "+record.Review {
		t.Fatalf("merge parents = %s", got)
	}
	if got := git(main, "rev-parse", record.Review+"^"); got != candidateID {
		t.Fatalf("review commit parent = %s", got)
	}
	if got := git(main, "diff", "--name-only", candidateID, record.Review); got != folder+"/acceptance.json" {
		t.Fatalf("review commit changed unexpected paths: %s", got)
	}
	var committed api.Decision
	if err := json.Unmarshal([]byte(git(main, "show", record.Review+":"+folder+"/acceptance.json")), &committed); err != nil || committed.ID != "fixture-accept" || committed.Signer != "Scratch Reviewer" || committed.Sequence != acceptedReceipt.Sequence {
		t.Fatalf("committed fixture acceptance mismatch: %+v (%v)", committed, err)
	}
	for _, oid := range []string{record.Review, record.Merge} {
		if message := git(main, "show", "-s", "--format=%B", oid); !strings.Contains(message, config.LandingTrailerLiterals[0]) {
			t.Fatalf("missing fixture trailer in %s: %s", oid, message)
		}
	}
	remoteSHA := strings.Fields(git(main, "ls-remote", "origin", "refs/heads/main"))[0]
	if remoteSHA != record.Merge || git(main, "rev-parse", "HEAD") != record.Merge || git(slot, "rev-parse", "HEAD") != record.Merge {
		t.Fatalf("remote/main/Outcome worktree did not converge on merge %s", record.Merge)
	}
	var outcome material.Outcome
	readScratchJSON(t, filepath.Join(record.Outcome, "run.json"), &outcome)
	var measurement struct{ Commit, Tree, Base, Status string }
	readScratchJSON(t, filepath.Join(record.Outcome, "fixture.json"), &measurement)
	if outcome.Commit != record.Merge || measurement.Commit != record.Merge || measurement.Tree != git(main, "rev-parse", record.Merge+"^{tree}") || measurement.Base != base || measurement.Status != "passed" {
		t.Fatalf("Outcome did not measure the actual landed tree: %+v / %+v", outcome, measurement)
	}
	documents := landed.Candidate.Documents
	if len(documents) != 3 || documents[0].ID != "fixture-notice" {
		t.Fatalf("expected fixture notice, Outcome index and report, got %+v", documents)
	}
	if documents[1].Kind != "outcome-index" || !strings.Contains(documents[1].Text, record.Merge) ||
		documents[2].Kind != "outcome" || !strings.Contains(documents[2].Text, record.Merge) {
		t.Fatal("landed-tree Outcome index and report were not returned through the production coordinator")
	}
	scratchRequest(t, ctx, "GET", server.URL+"/api/views?prefix=ledger&prefix=slot/fixture-slot", nil, http.StatusOK, &frame)
	var finalLedger ledger.Ledger
	if err := json.Unmarshal(frame.Puts["ledger"], &finalLedger); err != nil || finalLedger.Members[uid].Settlement != "landed" {
		t.Fatalf("missing landed ledger settlement: %+v (%v)", finalLedger, err)
	}
	var finalSlot map[string]string
	if err := json.Unmarshal(frame.Puts["slot/fixture-slot"], &finalSlot); err != nil || finalSlot["proposal"] != "" {
		t.Fatalf("completed Outcome failed to release the slot: %+v (%v)", finalSlot, err)
	}
	scratchRequest(t, ctx, "GET", server.URL+"/api/views?prefix=event/"+uid+"/", nil, http.StatusOK, &frame)
	landingTasks := map[string]string{}
	for _, raw := range frame.Puts {
		var event struct {
			Kind   string
			Task   struct{ ID, Phase string }
			Result struct{ OK bool }
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == "task-completed" && (event.Task.Phase == "land" || event.Task.Phase == "land-outcome") {
			if !event.Result.OK || landingTasks[event.Task.Phase] != "" {
				t.Fatalf("landing phase failed or repeated: %+v", event)
			}
			landingTasks[event.Task.Phase] = event.Task.ID
		}
	}
	if len(landingTasks) != 2 || finalLedger.Members[uid].Source != "task/"+landingTasks["land"] {
		t.Fatalf("landing and measurement were not distinct successful tasks: %+v", landingTasks)
	}
	for _, directory := range []string{main, slot} {
		if status := git(directory, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
			t.Fatalf("dirty scratch checkout after landing: %s\n%s", directory, status)
		}
	}
	evidence := map[string]any{"fixture": true, "root": root, "base": base, "candidate": candidateID, "review": record.Review, "merge": record.Merge, "remote": remote, "remote_sha": remoteSHA, "outcome": record.Outcome, "outcome_commit": measurement.Commit, "outcome_tree": measurement.Tree, "decision_feed": feed, "sign_next_with_unruled_doubt": first.Next, "workflow_status": landed.Status, "landing_tasks": landingTasks, "highlighted_paths": highlightedPaths}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "evidence.json"), string(data)+"\n", 0600)
	t.Logf("base=%s candidate=%s review=%s merge=%s remote=%s outcome=%s", base, candidateID, record.Review, record.Merge, remoteSHA, record.Outcome)
}

type scratchLandingRunner struct {
	candidate api.Candidate
	lander    *landing.Lander
}

func (r *scratchLandingRunner) Run(ctx context.Context, task workflow.Task) (workflow.Result, error) {
	switch task.Phase {
	case "land", "land-outcome":
		return r.lander.Run(ctx, task)
	case "worker", "wrapup":
		candidate := r.candidate
		return workflow.Result{OK: true, Candidate: &candidate, Detail: "FIXTURE role material"}, nil
	case "preflight", "worker-step", "project", "outcome", "commissar", "reviewer", "nitpicker", "wrap-gate", "surfaces":
		return workflow.Result{OK: true, Detail: "FIXTURE role or gate"}, nil
	default:
		return workflow.Result{}, fmt.Errorf("unexpected fixture phase %s", task.Phase)
	}
}

func startScratchDecision(t *testing.T, ctx context.Context, root, repository string) string {
	t.Helper()
	binary := os.Getenv("PROPOSAL_DECISION_BINARY")
	if binary == "" {
		binary = filepath.Join(root, "proposal-decision")
		build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/proposal-decision")
		build.Dir = "../proposal-decision"
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build decision server: %v\n%s", err, out)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(root, "decision.log"))
	if err != nil {
		t.Fatal(err)
	}
	peekScript, err := filepath.Abs("../../../../webui/apps/proposal-decision/server/peek.ts")
	if err != nil {
		log.Close()
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, binary, "--listen", address, "--repository", repository, "--database", filepath.Join(root, "decision.sqlite"), "--peek-script", peekScript)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Signal(os.Interrupt); _ = command.Wait(); _ = log.Close() })
	client := &http.Client{Timeout: 100 * time.Millisecond}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get("http://" + address + "/api/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return "http://" + address
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("decision server did not become ready; see %s", filepath.Join(root, "decision.log"))
	return ""
}

func scratchRequest(t *testing.T, ctx context.Context, method, url string, input any, status int, output any) {
	t.Helper()
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s %s: expected HTTP %d, got %d: %s", method, url, status, response.StatusCode, data)
	}
	if output != nil {
		if err := json.Unmarshal(data, output); err != nil {
			t.Fatalf("decode %s: %v\n%s", url, err, data)
		}
	}
}

func waitScratchProposal(t *testing.T, ctx context.Context, url, uid, status string) workflow.Proposal {
	t.Helper()
	var proposal workflow.Proposal
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		var frame api.Frame
		scratchRequest(t, ctx, "GET", url+"/api/views?prefix=proposal/"+uid, nil, http.StatusOK, &frame)
		if err := json.Unmarshal(frame.Puts["proposal/"+uid], &proposal); err != nil {
			t.Fatal(err)
		}
		if proposal.Status == status {
			return proposal
		}
		if proposal.Status == "stuck" || proposal.Status == "aborted" || strings.HasSuffix(proposal.Status, "-failed") {
			t.Fatalf("workflow stopped in %s/%s: %s", proposal.Status, proposal.Phase, proposal.Reason)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waiting for %s; last proposal %s/%s: %s", status, proposal.Status, proposal.Phase, proposal.Reason)
	return proposal
}

func readScratchJSON(t *testing.T, path string, output any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		t.Fatal(err)
	}
}
