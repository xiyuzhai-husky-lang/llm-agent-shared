// Package tab is the Diff tab of the diff/advanced correspondence form over one snapshot of a range: the index computed
// once, the views computed from it, the MUST SIGN signatures, and the caches that keep every answer from being
// computed twice.
package tab

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

type headingPart struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

// View is the Diff tab as the page holds it: the top of the tree with its counts and what is not per file.
type View struct {
	CandidateCommit string              `json:"candidate_commit"`
	Heading         []headingPart       `json:"heading"`
	Generation      uint64              `json:"generation"`
	Tree            []kindGroupView     `json:"tree"`
	FirstPosition   *diffPosition       `json:"first_position"`
	SignPresses     uint64              `json:"sign_presses"`
	Write           *hunkSignatureWrite `json:"write"`
}

// Citation is a citation of the opened file: candidate-side line `line`, up to `end_line` for a range.
type Citation struct {
	Line    uint32  `json:"line"`
	EndLine *uint32 `json:"end_line"`
}

// Query is what the page asks the Diff tab for besides its view: exactly one of the three.
type Query struct {
	Children *ChildrenQuery `json:"children,omitempty"`
	File     *FileQuery     `json:"file,omitempty"`
	Document *DocumentQuery `json:"document,omitempty"`
}

type ChildrenQuery struct {
	Group     string `json:"group"`
	Directory string `json:"directory"`
}

type FileQuery struct {
	Path  string    `json:"path"`
	Cited *Citation `json:"cited"`
}

type DocumentQuery struct {
	Path string `json:"path"`
}

// decodeVariant decodes an externally tagged value: an object of exactly one key, the variant, whose value carries
// exactly the variant's fields.
func decodeVariant(text []byte, variants map[string]func(json.RawMessage) error) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(text, &object); err != nil {
		return err
	}
	if len(object) != 1 {
		return fmt.Errorf("expected one variant, found %s", text)
	}
	for name, body := range object {
		decode, known := variants[name]
		if !known {
			return fmt.Errorf("unknown variant %q", name)
		}
		return decode(body)
	}
	return nil
}

func strictInto[Value any](required []string, target **Value) func(json.RawMessage) error {
	return func(body json.RawMessage) error {
		if err := requireFieldsAllowingNull(body, required); err != nil {
			return err
		}
		*target = new(Value)
		return decodeStrict(body, *target)
	}
}

// requireFieldsAllowingNull fails unless the object carries every one of `fields`; a field may be null.
func requireFieldsAllowingNull(text []byte, fields []string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(text, &object); err != nil {
		return err
	}
	if object == nil {
		return fmt.Errorf("expected an object, found null")
	}
	for _, field := range fields {
		if _, present := object[field]; !present {
			return fmt.Errorf("missing field `%s`", field)
		}
	}
	return nil
}

func (query *Query) UnmarshalJSON(text []byte) error {
	*query = Query{}
	return decodeVariant(text, map[string]func(json.RawMessage) error{
		"children": strictInto([]string{"group", "directory"}, &query.Children),
		"file": func(body json.RawMessage) error {
			if err := strictInto([]string{"path"}, &query.File)(body); err != nil {
				return err
			}
			if query.File.Cited != nil {
				encoded, _ := json.Marshal(query.File.Cited)
				return requireFields(encoded, []string{"line"})
			}
			return nil
		},
		"document": strictInto([]string{"path"}, &query.Document),
	})
}

// Key is the query in one canonical spelling, which keys the caches.
func (query *Query) Key() string {
	encoded, err := json.Marshal(query)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// CompareQueries orders queries as the Rust tab ordered them: rows, then files, then documents, each by its fields.
func CompareQueries(left, right *Query) int {
	rank := func(query *Query) int {
		switch {
		case query.Children != nil:
			return 0
		case query.File != nil:
			return 1
		}
		return 2
	}
	if ordering := cmp.Compare(rank(left), rank(right)); ordering != 0 {
		return ordering
	}
	switch {
	case left.Children != nil:
		return cmp.Or(strings.Compare(left.Children.Group, right.Children.Group), strings.Compare(left.Children.Directory, right.Children.Directory))
	case left.File != nil:
		if ordering := strings.Compare(left.File.Path, right.File.Path); ordering != 0 {
			return ordering
		}
		a, b := left.File.Cited, right.File.Cited
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		case b == nil:
			return 1
		}
		if ordering := cmp.Compare(a.Line, b.Line); ordering != 0 {
			return ordering
		}
		switch {
		case a.EndLine == nil && b.EndLine == nil:
			return 0
		case a.EndLine == nil:
			return -1
		case b.EndLine == nil:
			return 1
		}
		return cmp.Compare(*a.EndLine, *b.EndLine)
	}
	return strings.Compare(left.Document.Path, right.Document.Path)
}

// SignatureDependent is whether a signature changes the query's answer: rows and file views, not documents.
func (query *Query) SignatureDependent() bool { return query.Document == nil }

// Action is what a press on the page asks the Diff tab to do: exactly one of the four.
type Action struct {
	SendToPeek   *struct{ Path, Hunk, Text string }
	Sign         *struct{ Path, Hunk string }
	SignAllUnder *struct{ Path, Directory string }
	Comment      *struct{ Path, Hunk, Text, Timestamp string }
}

func (action *Action) UnmarshalJSON(text []byte) error {
	*action = Action{}
	return decodeVariant(text, map[string]func(json.RawMessage) error{
		"SendToPeek": func(body json.RawMessage) error {
			var fields struct {
				Path string `json:"path"`
				Hunk string `json:"hunk"`
				Text string `json:"text"`
			}
			if err := requireFields(body, []string{"path", "hunk", "text"}); err != nil {
				return err
			}
			if err := decodeStrict(body, &fields); err != nil {
				return err
			}
			action.SendToPeek = &struct{ Path, Hunk, Text string }{fields.Path, fields.Hunk, fields.Text}
			return nil
		},
		"Sign": func(body json.RawMessage) error {
			var fields struct {
				Path string `json:"path"`
				Hunk string `json:"hunk"`
			}
			if err := requireFields(body, []string{"path", "hunk"}); err != nil {
				return err
			}
			if err := decodeStrict(body, &fields); err != nil {
				return err
			}
			action.Sign = &struct{ Path, Hunk string }{fields.Path, fields.Hunk}
			return nil
		},
		"SignAllUnder": func(body json.RawMessage) error {
			var fields struct {
				Path      string `json:"path"`
				Directory string `json:"directory"`
			}
			if err := requireFields(body, []string{"path", "directory"}); err != nil {
				return err
			}
			if err := decodeStrict(body, &fields); err != nil {
				return err
			}
			action.SignAllUnder = &struct{ Path, Directory string }{fields.Path, fields.Directory}
			return nil
		},
		"Comment": func(body json.RawMessage) error {
			var fields struct {
				Path      string `json:"path"`
				Hunk      string `json:"hunk"`
				Text      string `json:"text"`
				Timestamp string `json:"timestamp"`
			}
			if err := requireFields(body, []string{"path", "hunk", "text", "timestamp"}); err != nil {
				return err
			}
			if err := decodeStrict(body, &fields); err != nil {
				return err
			}
			action.Comment = &struct{ Path, Hunk, Text, Timestamp string }{fields.Path, fields.Hunk, fields.Text, fields.Timestamp}
			return nil
		},
	})
}

// Signs is whether the action records signatures.
func (action *Action) Signs() bool { return action.Sign != nil || action.SignAllUnder != nil }

type citedView struct {
	Line  uint32   `json:"line"`
	Hunk  *string  `json:"hunk"`
	Lines []uint32 `json:"lines"`
}

type childrenAnswer struct {
	Generation uint64         `json:"generation"`
	Nodes      []treeNodeView `json:"nodes"`
}

type mustSignReasonView struct {
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type hunkView struct {
	Header              string                      `json:"header"`
	HeaderLine          string                      `json:"header_line"`
	Label               string                      `json:"label"`
	MustSignHunkOrdinal *uint32                     `json:"must_sign_hunk_ordinal"`
	Signature           *mustSignHunkSignatureState `json:"signature"`
	NeedsAction         bool                        `json:"needs_action"`
	Up                  *diffPosition               `json:"up"`
	Down                *diffPosition               `json:"down"`
	NextAction          *diffPosition               `json:"next_action"`
}

type batchSignOperation struct {
	Directory     string `json:"directory"`
	Label         string `json:"label"`
	UnsignedCount int    `json:"unsigned_count"`
}

type batchSignMenu struct {
	Operations []batchSignOperation `json:"operations"`
}

type fileAnswer struct {
	Generation      uint64              `json:"generation"`
	Path            string              `json:"path"`
	Labels          fileLabels          `json:"labels"`
	MustSignReason  *mustSignReasonView `json:"must_sign_reason"`
	TreeGroup       string              `json:"tree_group"`
	TreeDirectories []string            `json:"tree_directories"`
	Hunks           []hunkView          `json:"hunks"`
	Cited           *citedView          `json:"cited"`
	NextAction      *diffPosition       `json:"next_action"`
	BatchSignMenu   *batchSignMenu      `json:"batch_sign_menu"`
}

// Answer is the answer to one query: exactly one of the three, written externally tagged.
type Answer struct {
	children *childrenAnswer
	file     *fileAnswer
	document *fileDocument
}

func (answer Answer) MarshalJSON() ([]byte, error) {
	switch {
	case answer.children != nil:
		return marshalCompact(map[string]any{"children": answer.children})
	case answer.file != nil:
		return marshalCompact(map[string]any{"file": answer.file})
	}
	return marshalCompact(map[string]any{"document": answer.document})
}

// Generation is the view generation rows or a file view were answered at; a document stands for the whole snapshot.
func (answer Answer) Generation() (uint64, bool) {
	switch {
	case answer.children != nil:
		return answer.children.Generation, true
	case answer.file != nil:
		return answer.file.Generation, true
	}
	return 0, false
}

func marshalCompact(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// MarshalCompact is the JSON of a value as the page reads it, with no HTML escaping.
func MarshalCompact(value any) ([]byte, error) { return marshalCompact(value) }

// Refused is the page asking for something this Diff does not have, or an action it cannot take.
type Refused struct{ Reason string }

func (refused Refused) Error() string { return refused.Reason }

func refuse(format string, arguments ...any) error {
	return Refused{Reason: fmt.Sprintf(format, arguments...)}
}

// hunkPositions is where every hunk of the Diff stands in the order the tab walks, built once per snapshot.
type hunkPositions struct {
	positions        []diffPosition
	firstIndexOfFile map[string]int
}

func newHunkPositions(diff *rangeDiff) *hunkPositions {
	positions := &hunkPositions{firstIndexOfFile: map[string]int{}}
	for _, file := range diff.filesInDisplayOrder() {
		if len(file.hunks) > 0 {
			positions.firstIndexOfFile[file.path] = len(positions.positions)
		}
		for _, hunk := range file.hunks {
			positions.positions = append(positions.positions, diffPosition{Path: file.path, Hunk: hunk.header})
		}
	}
	return positions
}

func (positions *hunkPositions) at(index int) *diffPosition {
	position := positions.positions[index]
	return &position
}

// signatureIndex is the signature standing on each MUST SIGN hunk and the hunks that still ask for one.
type signatureIndex struct {
	byHunk        map[[2]string]mustSignHunkSignatureState
	askingIndices []int
	asking        map[int]bool
}

func newSignatureIndex(signatures *hunkSignatures, positions *hunkPositions) *signatureIndex {
	index := &signatureIndex{byHunk: map[[2]string]mustSignHunkSignatureState{}, asking: map[int]bool{}}
	askingHunks := map[[2]string]bool{}
	for _, hunk := range signatures.hunks {
		index.byHunk[[2]string{hunk.Path, hunk.Range}] = hunk
		if hunk.Signature == nil {
			askingHunks[[2]string{hunk.Path, hunk.Range}] = true
		}
	}
	for position, entry := range positions.positions {
		if askingHunks[[2]string{entry.Path, entry.Hunk}] {
			index.askingIndices = append(index.askingIndices, position)
			index.asking[position] = true
		}
	}
	return index
}

// nextAction is the first asking position after `current`, wrapping round, other than `current` itself.
func (index *signatureIndex) nextAction(positions *hunkPositions, current int) *diffPosition {
	start := current + 1
	split, _ := slices.BinarySearch(index.askingIndices, start)
	for _, candidate := range append(slices.Clone(index.askingIndices[split:]), index.askingIndices[:split]...) {
		if candidate != current {
			return positions.at(candidate)
		}
	}
	return nil
}

// nextGeneration is a number no earlier view of this server or of an earlier server has carried.
var generationCounter = func() *atomic.Uint64 {
	counter := &atomic.Uint64{}
	counter.Store(uint64(time.Now().UnixMilli()) * 1000)
	return counter
}()

func nextGeneration() uint64 { return generationCounter.Add(1) - 1 }

// DeclaredMustSign is what the instance directory declares as MUST SIGN: `must-sign-patterns.txt`, which may be absent,
// and `must-sign-additions.json`, which must be present and list at least one addition.
type DeclaredMustSign struct {
	policy        mustSignPolicy
	additionsFile string
}

func ReadDeclaredMustSign(instanceDirectory string) (*DeclaredMustSign, error) {
	declared := &DeclaredMustSign{additionsFile: filepath.Join(instanceDirectory, mustSignAdditionsFile)}
	patternsFile := filepath.Join(instanceDirectory, mustSignPatternsFile)
	text, err := os.ReadFile(patternsFile)
	switch {
	case err == nil:
		patterns, err := mustSignPatternsFromText(string(text))
		if err != nil {
			return nil, err
		}
		declared.policy.patterns = patterns
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("cannot read the must-sign patterns file %s: %v", patternsFile, err)
	}
	additions, err := readMustSignAdditions(declared.additionsFile)
	if err != nil {
		return nil, err
	}
	declared.policy.additions = additions
	return declared, nil
}

// RangeTab is the Diff tab over one snapshot of the range.
type RangeTab struct {
	candidateKind      candidateKind
	sidecarFile        string
	diff               *rangeDiff
	positions          *hunkPositions
	signatures         *hunkSignatures
	signatureIndex     *signatureIndex
	signPresses        uint64
	lastWrite          *hunkSignatureWrite
	view               View
	loadedFiles        map[string]*diffFile
	documents          map[string]*fileDocument
	signatureDependent map[string]Answer
}

// ComputeRangeTab computes the index of `base..candidate` under the declared MUST SIGN parts and reads the signatures
// in `sidecarFile`.
func ComputeRangeTab(repository *GitRepository, base, candidate string, workingTree bool, sidecarFile string, declared *DeclaredMustSign) (*RangeTab, error) {
	kind := candidateCommit
	if workingTree {
		kind = candidateWorkingTree
	}
	diff, err := computeRangeIndex(repository, base, candidate, &declared.policy, declared.additionsFile)
	if err != nil {
		return nil, err
	}
	signatures, err := readRangeSignatures(sidecarFile, candidate, kind, diff)
	if err != nil {
		return nil, err
	}
	tab := &RangeTab{candidateKind: kind, sidecarFile: sidecarFile, diff: diff, positions: newHunkPositions(diff),
		signatures: signatures, loadedFiles: map[string]*diffFile{}, documents: map[string]*fileDocument{},
		signatureDependent: map[string]Answer{}}
	tab.signatureIndex = newSignatureIndex(signatures, tab.positions)
	tab.view = tab.viewOf()
	return tab, nil
}

func (tab *RangeTab) View() View { return tab.view }

// ChangedPaths is every changed path of the range, in the order the index lists them.
func (tab *RangeTab) ChangedPaths() []string {
	paths := make([]string, 0, len(tab.diff.files))
	for _, file := range tab.diff.files {
		paths = append(paths, file.path)
	}
	return paths
}

// Answer answers a query from the caches, computing it on first request.
func (tab *RangeTab) Answer(repository *GitRepository, query *Query) (Answer, error) {
	if query.Document != nil {
		path := query.Document.Path
		if document, known := tab.documents[path]; known {
			return Answer{document: document}, nil
		}
		loaded, err := tab.loadedFile(repository, path)
		if err != nil {
			return Answer{}, err
		}
		document, err := documentOf(repository, tab.diff, loaded)
		if err != nil {
			return Answer{}, err
		}
		tab.documents[path] = document
		return Answer{document: document}, nil
	}
	key := query.Key()
	if answer, known := tab.signatureDependent[key]; known {
		return answer, nil
	}
	generation := tab.view.Generation
	var answer Answer
	if query.Children != nil {
		nodes, found := childrenView(tab.diff, tab.signatures, query.Children.Group, query.Children.Directory)
		if !found {
			return Answer{}, refuse("%q is not a directory of the tree group %q", query.Children.Directory, query.Children.Group)
		}
		answer = Answer{children: &childrenAnswer{Generation: generation, Nodes: nodes}}
	} else {
		path, cited := query.File.Path, query.File.Cited
		if cited != nil && (cited.Line == 0 || cited.EndLine != nil && *cited.EndLine < cited.Line) {
			endLine := "None"
			if cited.EndLine != nil {
				endLine = fmt.Sprintf("Some(%d)", *cited.EndLine)
			}
			return Answer{}, refuse("%q: cited lines %d–%s are not a line range", path, cited.Line, endLine)
		}
		file := tab.fileView(generation, path, cited)
		if file == nil {
			return Answer{}, refuse("%q is not a file of this Diff", path)
		}
		answer = Answer{file: file}
	}
	tab.signatureDependent[key] = answer
	return answer, nil
}

// InitialQueries is the queries a page opening on no address asks first: the rows of the directories holding the first
// file of the tree, the file and its highlighted document.
func (tab *RangeTab) InitialQueries() []*Query {
	first := tab.view.FirstPosition
	if first == nil {
		return nil
	}
	group, directories := treeLocation(tab.diff, tab.diff.file(first.Path))
	var queries []*Query
	for _, directory := range directories {
		queries = append(queries, &Query{Children: &ChildrenQuery{Group: group, Directory: directory}})
	}
	return append(queries, &Query{File: &FileQuery{Path: first.Path}}, &Query{Document: &DocumentQuery{Path: first.Path}})
}

func (tab *RangeTab) loadedFile(repository *GitRepository, path string) (*diffFile, error) {
	if loaded, known := tab.loadedFiles[path]; known {
		return loaded, nil
	}
	file := tab.diff.file(path)
	if file == nil {
		return nil, refuse("%q is not a file of this Diff", path)
	}
	loaded, err := loadFile(repository, tab.diff, file)
	if err != nil {
		return nil, err
	}
	tab.loadedFiles[path] = loaded
	return loaded, nil
}

// Apply applies one press of the page. A signature re-reads the sidecar and moves the view to a new generation; Ask
// Peek answers the question to queue.
func (tab *RangeTab) Apply(repository *GitRepository, action *Action, signer string) (string, bool, error) {
	candidate := tab.diff.candidate
	switch {
	case action.Sign != nil:
		write, err := signRangeMustSignHunk(tab.sidecarFile, candidate, tab.candidateKind, tab.diff, action.Sign.Path, action.Sign.Hunk, signer, time.Now())
		if err != nil {
			return "", false, err
		}
		tab.signPresses++
		tab.lastWrite = &write
		return "", false, tab.signaturesChanged()
	case action.SignAllUnder != nil:
		path, directory := action.SignAllUnder.Path, action.SignAllUnder.Directory
		hasMustSign := slices.ContainsFunc(tab.signatures.hunks, func(hunk mustSignHunkSignatureState) bool { return hunk.Path == path })
		isAncestor := false
		for index := range len(path) {
			if path[index] == '/' && path[:index] == directory {
				isAncestor = true
			}
		}
		if !hasMustSign || !isAncestor {
			return "", false, refuse("Not an ancestor directory of a MUST SIGN file: path=%q, directory=%q", path, directory)
		}
		if err := signRangeUnsignedHunksUnderDirectory(tab.sidecarFile, candidate, tab.candidateKind, tab.diff, directory, signer, time.Now()); err != nil {
			return "", false, err
		}
		tab.signPresses++
		return "", false, tab.signaturesChanged()
	case action.Comment != nil:
		return "", false, refuse("A commit range has no review to send to; use Ask Peek")
	}
	path, header, text := action.SendToPeek.Path, action.SendToPeek.Hunk, action.SendToPeek.Text
	if strings.TrimSpace(text) == "" {
		return "", false, refuse("Write a comment first; path=%q, hunk=%q, text=%q", path, header, text)
	}
	loaded, err := tab.loadedFile(repository, path)
	if err != nil {
		return "", false, err
	}
	index := slices.IndexFunc(loaded.hunks, func(hunk hunkFields) bool { return hunk.header == header })
	if index < 0 {
		return "", false, refuse("The requested hunk is not in this range: path=%q, hunk=%q", path, header)
	}
	found := &loaded.hunks[index]
	return fmt.Sprintf("`%s` `%s`\n\n```diff\n%s\n```\n\n%s", path, found.headerLine, found.commentUnifiedText, text), true, nil
}

func (tab *RangeTab) signaturesChanged() error {
	signatures, err := readRangeSignatures(tab.sidecarFile, tab.diff.candidate, tab.candidateKind, tab.diff)
	if err != nil {
		return err
	}
	tab.signatures = signatures
	tab.signatureIndex = newSignatureIndex(signatures, tab.positions)
	tab.view = tab.viewOf()
	tab.signatureDependent = map[string]Answer{}
	return nil
}

func (tab *RangeTab) viewOf() View {
	statistics := tab.diff.statistics
	separator := " · "
	view := View{
		CandidateCommit: tab.diff.candidate,
		Heading: []headingPart{
			{"Diff", "heading"}, {separator, "separator"}, {fmt.Sprint(statistics.files), "neutral"}, {separator, "separator"},
			{fmt.Sprintf("+%d", statistics.additions), "positive"}, {"/", "separator"},
			{fmt.Sprintf("−%d", statistics.deletions), "negative"}, {separator, "separator"},
			{fmt.Sprintf("MUST SIGN %d/%d", tab.signatures.progress.signedHunks, tab.signatures.progress.totalHunks), "must_sign"},
		},
		Generation:  nextGeneration(),
		Tree:        treeViewOf(tab.diff, tab.signatures),
		SignPresses: tab.signPresses,
		Write:       tab.lastWrite,
	}
	if files := tab.diff.filesInDisplayOrder(); len(files) > 0 {
		position := firstPosition(files[0])
		view.FirstPosition = &position
	}
	return view
}

// fileView is one opened file: its header words, where the tree holds it, and each hunk with its signature, whether it
// still asks for one, and its neighbours in the order the tab walks every hunk of the Diff.
func (tab *RangeTab) fileView(generation uint64, path string, cited *Citation) *fileAnswer {
	file := tab.diff.file(path)
	if file == nil {
		return nil
	}
	positions, index := tab.positions, tab.signatureIndex
	firstIndex := positions.firstIndexOfFile[file.path]
	hunks := []hunkView{}
	for offset := range file.hunks {
		hunk := &file.hunks[offset]
		position := firstIndex + offset
		view := hunkView{Header: hunk.header, HeaderLine: hunk.headerLine, Label: fmt.Sprintf("#%d", offset+1),
			NeedsAction: index.asking[position], NextAction: index.nextAction(positions, position)}
		if hunk.mustSignOrdinal != 0 {
			view.Label = fmt.Sprintf("#%d", hunk.mustSignOrdinal)
			view.MustSignHunkOrdinal = pointer(hunk.mustSignOrdinal)
		}
		if state, known := index.byHunk[[2]string{file.path, hunk.header}]; known {
			view.Signature = &state
		}
		if position > 0 {
			view.Up = positions.at(position - 1)
		}
		if position+1 < len(positions.positions) {
			view.Down = positions.at(position + 1)
		}
		hunks = append(hunks, view)
	}
	group, directories := treeLocation(tab.diff, file)
	answer := &fileAnswer{Generation: generation, Path: file.path, Labels: labelsOf(file), TreeGroup: group,
		TreeDirectories: directories, Hunks: hunks, NextAction: index.nextAction(positions, -1)}
	if file.mustSignReason != "" {
		answer.MustSignReason = &mustSignReasonView{Label: "MUST SIGN · must-sign-additions.json", Reason: file.mustSignReason}
	}
	if cited != nil {
		view := &citedView{Line: cited.Line, Lines: []uint32{}}
		for hunkIndex := range file.hunks {
			hunk := &file.hunks[hunkIndex]
			if hunk.newStart <= cited.Line && cited.Line < hunk.newStart+hunk.newLines {
				view.Hunk = pointer(hunk.header)
				break
			}
		}
		if view.Hunk == nil && len(file.hunks) > 0 {
			view.Hunk = pointer(file.hunks[0].header)
		}
		end := cited.Line
		if cited.EndLine != nil {
			end = *cited.EndLine
		}
		for line := cited.Line; line <= end; line++ {
			view.Lines = append(view.Lines, line)
		}
		answer.Cited = view
	}
	if slices.ContainsFunc(tab.signatures.hunks, func(hunk mustSignHunkSignatureState) bool { return hunk.Path == file.path }) {
		answer.BatchSignMenu = tab.batchSignMenu(file.path)
	}
	return answer
}

// batchSignMenu is the Sign menu of the MUST SIGN file `path`: its ancestor directories from the outermost to its
// parent, each with the unsigned MUST SIGN hunks under it.
func (tab *RangeTab) batchSignMenu(path string) *batchSignMenu {
	menu := &batchSignMenu{Operations: []batchSignOperation{}}
	for index := range len(path) {
		if path[index] != '/' {
			continue
		}
		directory, prefix := path[:index], path[:index+1]
		unsigned := 0
		for _, hunk := range tab.signatures.hunks {
			if hunk.Signature == nil && strings.HasPrefix(hunk.Path, prefix) {
				unsigned++
			}
		}
		menu.Operations = append(menu.Operations, batchSignOperation{Directory: directory, Label: "under " + directory, UnsignedCount: unsigned})
	}
	return menu
}
