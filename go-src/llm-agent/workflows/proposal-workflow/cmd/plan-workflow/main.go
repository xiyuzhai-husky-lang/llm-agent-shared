package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"proposal-workflow/internal/cli"
	"proposal-workflow/internal/plan"
	"proposal-workflow/internal/project"
)

func main() {
	repo := flag.String("repo", "", "tools checkout")
	mainRoot := flag.String("main", os.Getenv("PROPOSAL_MAIN"), "state-owning checkout")
	projectPath := flag.String("project", os.Getenv("PROPOSAL_PROJECT"), "project path")
	port := flag.Int("port", 4318, "plan resident port")
	web := flag.String("web", "", "plan frontend")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("serve|start|answer|status|reset|verdict required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if flag.Arg(0) == "serve" {
		if *repo == "" {
			out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
			if err != nil {
				log.Fatal(err)
			}
			*repo = strings.TrimSpace(string(out))
		}
		if *mainRoot == "" {
			*mainRoot = *repo
		}
		if *web == "" {
			*web = filepath.Join(*repo, "webui/apps/plan-workflow/dist")
		}
		config, err := project.Load(*repo, *mainRoot, *projectPath)
		if err != nil {
			log.Fatal(err)
		}
		options := cli.Options{Repository: *repo, Main: *mainRoot, Project: *projectPath}
		service, err := plan.Open(config, func(path string) error { return cli.CheckAssignment(options, path) })
		if err != nil {
			log.Fatal(err)
		}
		handler, err := service.Handler(ctx, *web)
		if err != nil {
			service.Close()
			log.Fatal(err)
		}
		server := http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", *port), Handler: handler, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		}()
		log.Printf("plan-workflow: http://%s", server.Addr)
		err = server.ListenAndServe()
		stop()
		closeErr := service.Close()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		if closeErr != nil {
			log.Fatal(closeErr)
		}
		return
	}
	flags := flag.NewFlagSet(flag.Arg(0), flag.ExitOnError)
	instance := flags.String("instance", "", "instance ID")
	assignment := flags.String("assignment", "", "assignment file")
	rank := flags.String("rank", "", "mission rank")
	startAt := flags.String("start-at", "planner", "planner or reviewer")
	text := flags.String("text", "", "user text")
	kind := flags.String("kind", "", "accept, modify or reject")
	flags.Parse(flag.Args()[1:])
	if *instance == "" || flags.NArg() != 0 {
		log.Fatal("--instance required; unexpected positional arguments")
	}
	var body any
	method, path := "POST", "/api/plans/"+*instance+"/"+flag.Arg(0)
	switch flag.Arg(0) {
	case "start":
		absolute, err := filepath.Abs(*assignment)
		if err != nil {
			log.Fatal(err)
		}
		body = plan.Start{Instance: *instance, Assignment: absolute, Rank: *rank, StartAt: *startAt}
		path = "/api/plans/start"
	case "answer":
		body = map[string]string{"text": *text}
	case "verdict":
		body = map[string]string{"kind": *kind, "text": *text}
	case "status":
		method = "GET"
	case "reset":
	default:
		log.Fatal("unknown command")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		log.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", *port, path), bytes.NewReader(raw))
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 35 * time.Second}).Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		log.Fatal(err)
	}
	if response.StatusCode != 200 {
		log.Fatalf("HTTP %d: %s", response.StatusCode, answer)
	}
	fmt.Println(string(answer))
}
