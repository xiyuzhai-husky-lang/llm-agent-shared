// Command diff-advanced is the diff/advanced correspondence server: a long-running process that holds a range's Diff
// and serves it to the page over the synchrotron, serves the built page, and runs the Peek server beside itself.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"diff-advanced/tab"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// signer is the one signer for now, as the proposal reviewer signs.
const signer = "Xiyu Zhai"

// instance is one `diff.json` and the directory that holds the MUST SIGN declarations, the signatures and the Peek
// chats.
type instance struct {
	title          string
	repositoryRoot string
	mergeBase      string
	commit         string
	workingTree    bool
	directory      string
}

func readInstance(diffFile string) (*instance, error) {
	diffFile, err := filepath.Abs(diffFile)
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(diffFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read the diff instance %s: %v", diffFile, err)
	}
	notAnInstance := func(reason any) error { return fmt.Errorf("%s is not a diff instance: %v", diffFile, reason) }
	var file struct {
		Title     string          `json:"title"`
		Repo      string          `json:"repo"`
		MergeBase string          `json:"merge_base"`
		Candidate json.RawMessage `json:"candidate"`
	}
	if err := decodeStrictWithFields(text, &file, "title", "repo", "merge_base", "candidate"); err != nil {
		return nil, notAnInstance(err)
	}
	var candidate struct {
		Kind   string  `json:"kind"`
		Commit *string `json:"commit"`
	}
	if err := decodeStrictWithFields(file.Candidate, &candidate, "kind"); err != nil {
		return nil, notAnInstance(err)
	}
	result := &instance{title: file.Title, mergeBase: file.MergeBase, directory: filepath.Dir(diffFile)}
	switch {
	case candidate.Kind == "commit" && candidate.Commit != nil:
		result.commit = *candidate.Commit
	case candidate.Kind == "working-tree" && candidate.Commit == nil:
		result.workingTree = true
	default:
		return nil, notAnInstance(fmt.Sprintf("candidate %s is neither {\"kind\": \"commit\", \"commit\"} nor {\"kind\": \"working-tree\"}", file.Candidate))
	}
	for _, field := range [][2]string{{"title", file.Title}, {"merge_base", file.MergeBase}} {
		if strings.TrimSpace(field[1]) == "" {
			return nil, fmt.Errorf("%s: %s is empty", diffFile, field[0])
		}
	}
	result.repositoryRoot = file.Repo
	if !filepath.IsAbs(file.Repo) {
		result.repositoryRoot = filepath.Join(result.directory, file.Repo)
	}
	return result, nil
}

func decodeStrictWithFields(text []byte, target any, required ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(text, &fields); err != nil {
		return err
	}
	for _, field := range required {
		if _, present := fields[field]; !present {
			return fmt.Errorf("missing field `%s`", field)
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(text)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// snapshot is the range at launch: its two ends resolved (the working tree written as a commit), the MUST SIGN
// declarations read, the index and the signatures computed, and the answers the first page asks for cached.
type snapshot struct {
	header         header
	headerJSON     []byte
	lock           sync.Mutex
	tab            *tab.RangeTab
	initialQueries []*tab.Query
}

func takeSnapshot(source *instance, repository *tab.GitRepository) (*snapshot, error) {
	var candidate string
	var err error
	if source.workingTree {
		candidate, err = repository.WorkingTreeSnapshotCommit()
	} else {
		candidate, err = repository.RevParseCommit(source.commit)
	}
	if err != nil {
		return nil, err
	}
	base, err := repository.RevParseCommit(source.mergeBase)
	if err != nil {
		return nil, err
	}
	declared, err := tab.ReadDeclaredMustSign(source.directory)
	if err != nil {
		return nil, err
	}
	sidecar := filepath.Join(source.directory, "signatures", candidate+".json")
	candidateShort := candidate[:11]
	if source.workingTree {
		sidecar = filepath.Join(source.directory, "signatures", "worktree.json")
		candidateShort = "working tree"
	}
	rangeTab, err := tab.ComputeRangeTab(repository, base, candidate, source.workingTree, sidecar, declared)
	if err != nil {
		return nil, err
	}
	taken := &snapshot{tab: rangeTab, initialQueries: rangeTab.InitialQueries(), header: header{
		Title: source.title, Selection: "advanced/" + base, Base: base, Candidate: candidate, BaseShort: base[:11],
		CandidateShort: candidateShort, ChangedPaths: rangeTab.ChangedPaths(),
	}}
	for _, query := range taken.initialQueries {
		if _, err := rangeTab.Answer(repository, query); err != nil {
			return nil, err
		}
	}
	taken.headerJSON, err = tab.MarshalCompact(taken.header)
	return taken, err
}

type server struct {
	repository *tab.GitRepository
	snapshot   *snapshot
	peekPort   int
	upgrader   websocket.Upgrader
}

func main() {
	instanceFile := flag.String("instance", "", "the instance's diff.json; its directory holds the MUST SIGN declarations, the signatures and the Peek chats")
	webRoot := flag.String("web-root", "", "the built page, webui/apps/diff-advanced/dist")
	peekServerScript := flag.String("peek-server-script", "", "the Peek server script, webui/apps/diff-advanced/server/peek-server.ts, run with Bun beside this server")
	port := flag.Int("port", 0, "the port to listen on, on 127.0.0.1")
	flag.Parse()
	if *instanceFile == "" || *webRoot == "" || *peekServerScript == "" || *port <= 0 || *port > 65535 || flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	source, err := readInstance(*instanceFile)
	if err != nil {
		log.Fatal(err)
	}
	repository := tab.NewGitRepository(source.repositoryRoot)
	// The snapshot and the first page's answers are computed before the server listens.
	taken, err := takeSnapshot(source, repository)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		log.Fatal(err)
	}
	served := &server{repository: repository, snapshot: taken, peekPort: *port + 1,
		upgrader: websocket.Upgrader{ReadBufferSize: 1 << 16, WriteBufferSize: 1 << 16,
			// The page is reached through a tunnel that rewrites Host, so the Origin never matches it; any origin is accepted.
			CheckOrigin: func(*http.Request) bool { return true }}}
	peek := exec.Command("bun", *peekServerScript, "--port", strconv.Itoa(served.peekPort), "--repository", source.repositoryRoot,
		"--instance-directory", source.directory, "--context", fmt.Sprintf("http://127.0.0.1:%d/api/peek-context", *port))
	peek.Stdout, peek.Stderr = os.Stdout, os.Stderr
	if err := peek.Start(); err != nil {
		log.Fatalf("cannot start the Peek server %s: %v", *peekServerScript, err)
	}
	go func() {
		err := peek.Wait()
		log.Fatalf("the Peek server exited: %v", err)
	}()
	fmt.Printf("http://127.0.0.1:%d\n", *port)
	if err := http.Serve(listener, served.routes(*webRoot)); err != nil {
		_ = peek.Process.Kill()
		log.Fatal(err)
	}
}

func (served *server) routes(webRoot string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", served.websocket)
	mux.HandleFunc("GET /api/peek-context", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(served.snapshot.headerJSON)
	})
	mux.HandleFunc("/api/diff-peek", served.peekProxy)
	mux.Handle("/", compressed(staticPage(webRoot)))
	return mux
}

// staticPage serves the built page, `index.html` for any path that names no file. Vite names every built asset by its
// content, so an asset is cached for good; the page itself is asked for afresh so a rebuild is seen at once.
func staticPage(webRoot string) http.Handler {
	files := http.FileServer(http.Dir(webRoot))
	index := filepath.Join(webRoot, "index.html")
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/assets/") {
			writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			writer.Header().Set("Cache-Control", "no-store")
		}
		path := filepath.Join(webRoot, filepath.FromSlash(filepathClean(request.URL.Path)))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(writer, request, index)
			return
		}
		files.ServeHTTP(writer, request)
	})
}

func filepathClean(path string) string {
	cleaned := filepath.ToSlash(filepath.Clean("/" + path))
	return strings.TrimPrefix(cleaned, "/")
}

// websocket is one page's connection: its synchrotron, and how far the page has been told.
func (served *server) websocket(writer http.ResponseWriter, request *http.Request) {
	socket, err := served.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer socket.Close()
	socket.SetReadLimit(1 << 20)
	connection := &connection{mirror: newSynchrotron()}
	for {
		messageType, text, err := socket.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		parsed, err := parseRequest(text)
		if err != nil {
			log.Printf("a page sent a request that is not a diff/advanced request: %v", err)
			return
		}
		answered := served.respond(connection, parsed)
		encoded, err := tab.MarshalCompact(answered)
		if err != nil {
			panic(err)
		}
		if err := socket.WriteMessage(websocket.TextMessage, encoded); err != nil {
			return
		}
	}
}

type connection struct {
	mirror      *synchrotron
	initialized bool
	sent        status
}

func (served *server) respond(connection *connection, parsed *request) response {
	var err error
	if parsed.Kind == "init" {
		err = served.initialize(connection)
	} else {
		err = served.takeViewAction(connection, parsed.ViewAction, *parsed.Status)
	}
	if err != nil {
		message := err.Error()
		return response{Kind: "refused", RequestIdentifier: parsed.RequestIdentifier, Message: &message}
	}
	diff := connection.mirror.actionsDiff(connection.sent)
	connection.sent = connection.mirror.status()
	return response{Kind: "actions_diff", RequestIdentifier: parsed.RequestIdentifier, Diff: &diff}
}

func mustTake(mirror *synchrotron, taken action) {
	if err := mirror.takeAction(taken); err != nil {
		panic(fmt.Sprintf("the server's own actions apply to its synchrotron: %v", err))
	}
}

func (served *server) initialize(connection *connection) error {
	if connection.initialized {
		return errors.New("the connection is already initialized")
	}
	taken := served.snapshot
	taken.lock.Lock()
	defer taken.lock.Unlock()
	view := taken.tab.View()
	headerValue := taken.header
	mustTake(connection.mirror, action{Kind: "initialize", Header: &headerValue, View: &view})
	// The answers the page asks for first travel with the view, from the tab's caches.
	for _, query := range taken.initialQueries {
		answer, err := taken.tab.Answer(served.repository, query)
		if err != nil {
			return err
		}
		mustTake(connection.mirror, action{Kind: "cache_answer", Query: query, Answer: &answer})
	}
	connection.initialized = true
	return nil
}

// takeViewAction adds the actions one view action takes to the connection's synchrotron; nothing is taken when the
// view action is refused.
func (served *server) takeViewAction(connection *connection, requested *viewAction, reported status) error {
	if !connection.initialized {
		return errors.New("Init is required")
	}
	// The page reports how many actions it holds; it may not have received the answers to requests still in flight.
	if reported.ActionsLen > connection.sent.ActionsLen {
		return errors.New("the page reports actions the server never sent")
	}
	taken := served.snapshot
	taken.lock.Lock()
	defer taken.lock.Unlock()
	mirror := connection.mirror
	var actions []action
	switch requested.Kind {
	case "ask":
		if mirror.answer(requested.Query, mirror.view.Generation) {
			return nil
		}
		answer, err := taken.tab.Answer(served.repository, requested.Query)
		if err != nil {
			return err
		}
		actions = append(actions, action{Kind: "cache_answer", Query: requested.Query, Answer: &answer})
	case "act":
		question, asksPeek, err := taken.tab.Apply(served.repository, requested.Action, signer)
		if err != nil {
			return err
		}
		if requested.Action.Signs() {
			view := taken.tab.View()
			actions = append(actions, action{Kind: "replace_view", View: &view})
			for _, query := range mirror.signatureDependentQueries() {
				answer, err := taken.tab.Answer(served.repository, query)
				if err != nil {
					return err
				}
				actions = append(actions, action{Kind: "cache_answer", Query: query, Answer: &answer})
			}
		}
		if asksPeek {
			actions = append(actions, action{Kind: "queue_peek_question", Question: &queuedPeekQuestion{Identifier: uuid.NewString(), Text: question}})
		}
	case "consume_peek_question":
		actions = append(actions, action{Kind: "consume_peek_question", Identifier: requested.Identifier})
	}
	for _, taken := range actions {
		if requested.Kind == "consume_peek_question" {
			if err := mirror.takeAction(taken); err != nil {
				return err
			}
			continue
		}
		mustTake(mirror, taken)
	}
	return nil
}

// peekProxy forwards Peek's requests to the Peek server and streams its replies back as they are written.
func (served *server) peekProxy(writer http.ResponseWriter, request *http.Request) {
	target := fmt.Sprintf("http://127.0.0.1:%d%s", served.peekPort, request.URL.RequestURI())
	forwarded, err := http.NewRequestWithContext(request.Context(), request.Method, target, request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}
	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		forwarded.Header.Set("Content-Type", contentType)
	}
	answer, err := http.DefaultTransport.RoundTrip(forwarded)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		http.Error(writer, "the Peek server did not answer: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer answer.Body.Close()
	for _, name := range []string{"Content-Type", "Allow", "X-Accel-Buffering"} {
		if value := answer.Header.Get(name); value != "" {
			writer.Header().Set(name, value)
		}
	}
	writer.WriteHeader(answer.StatusCode)
	flusher, _ := writer.(http.Flusher)
	buffer := make([]byte, 32*1024)
	for {
		read, err := answer.Body.Read(buffer)
		if read > 0 {
			if _, writeError := writer.Write(buffer[:read]); writeError != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
	}
}
