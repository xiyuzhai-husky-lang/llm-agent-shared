package tab

// The per-hunk signatures of the range: the sidecar `signatures/<candidate>.json` (or `signatures/worktree.json`) in
// the instance directory, read strictly, and the presses of the Sign button recorded under the rate limit.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// rangeSignatureFolder is the `folder` a range's signature document records: a range has no proposal folder.
const rangeSignatureFolder = ""

// signsTooFastMessage is what the reviewer is told when the rate limit refuses a signature.
const signsTooFastMessage = "You sign too fast. Are you serious?"

// changedLinesPerSecond is how many changed lines a second of waiting buys.
const changedLinesPerSecond = 100

type candidateKind int

const (
	// candidateCommit: the sidecar names that commit, and a signature is bound to its hunk key.
	candidateCommit candidateKind = iota
	// candidateWorkingTree: the sidecar names the snapshot it was last written at, and a signature is bound to the
	// content of the hunk it signed.
	candidateWorkingTree
)

type hunkSignatureContent struct {
	Body       []string `json:"body"`
	Occurrence uint32   `json:"occurrence"`
}

type hunkSignature struct {
	Path              string                `json:"path"`
	Hunk              string                `json:"hunk"`
	Signer            string                `json:"signer"`
	At                string                `json:"at"`
	Carried           *bool                 `json:"carried,omitempty"`
	SignedAtCandidate *string               `json:"signedAtCandidate,omitempty"`
	Content           *hunkSignatureContent `json:"content,omitempty"`
}

type hunkSignatureDocument struct {
	Candidate   string          `json:"candidate"`
	Folder      string          `json:"folder"`
	Signatures  []hunkSignature `json:"signatures"`
	CarriedFrom *string         `json:"carriedFrom,omitempty"`
}

// mustSignHunkSignatureState is one MUST SIGN hunk as the Diff tab shows it.
type mustSignHunkSignatureState struct {
	Path         string         `json:"path"`
	Hunk         string         `json:"hunk"`
	Range        string         `json:"range"`
	Ordinal      uint32         `json:"ordinal"`
	Total        uint32         `json:"total"`
	ChangedLines uint32         `json:"changedLines"`
	Signature    *hunkSignature `json:"signature,omitempty"`
}

type mustSignProgress struct {
	signedHunks uint32
	totalHunks  uint32
	signedFiles uint32
	totalFiles  uint32
}

// hunkSignatureWrite is what one press of the Sign button did.
type hunkSignatureWrite struct {
	Path                     string  `json:"path"`
	Hunk                     string  `json:"hunk"`
	Outcome                  string  `json:"outcome"`
	ChangedLines             uint32  `json:"changedLines"`
	RequiredWaitMilliseconds uint64  `json:"requiredWaitMilliseconds"`
	RemainingMilliseconds    *uint64 `json:"remainingMs,omitempty"`
	Message                  *string `json:"message,omitempty"`
}

// hunkSignatures is the signature state of the range: the sidecar document and one entry per MUST SIGN hunk.
type hunkSignatures struct {
	document hunkSignatureDocument
	hunks    []mustSignHunkSignatureState
	progress mustSignProgress
}

// mustSignHunk is one MUST SIGN hunk of the Diff in the form the signature machinery works in.
type mustSignHunk struct {
	path         string
	key          string
	header       string
	ordinal      uint32
	changedLines uint32
	bodyLines    []string
}

// signatureHunkKey is `<old start>:<old lines>:<new start>:<new lines>:<the @@ line Git wrote>`.
func signatureHunkKey(hunk *hunkFields) string {
	return fmt.Sprintf("%d:%d:%d:%d:%s", hunk.oldStart, hunk.oldLines, hunk.newStart, hunk.newLines, hunk.headerLine)
}

func mustSignHunksInDisplayOrder(diff *rangeDiff) []mustSignHunk {
	var hunks []mustSignHunk
	for fileIndex := range diff.files {
		file := &diff.files[fileIndex]
		for hunkIndex := range file.hunks {
			hunk := &file.hunks[hunkIndex]
			if hunk.mustSignOrdinal == 0 {
				continue
			}
			entry := mustSignHunk{path: file.path, key: signatureHunkKey(hunk), header: hunk.header, ordinal: hunk.mustSignOrdinal}
			for _, line := range hunk.lines {
				marker := map[lineKind]string{lineContext: " ", lineInsertion: "+", lineDeletion: "-"}[line.kind]
				entry.bodyLines = append(entry.bodyLines, marker+line.text)
				if line.kind != lineContext {
					entry.changedLines++
				}
			}
			if entry.bodyLines == nil {
				entry.bodyLines = []string{}
			}
			hunks = append(hunks, entry)
		}
	}
	slices.SortStableFunc(hunks, func(left, right mustSignHunk) int { return int(left.ordinal) - int(right.ordinal) })
	return hunks
}

func (hunk *mustSignHunk) sameContent(other *mustSignHunk) bool {
	return hunk.path == other.path && slices.Equal(hunk.bodyLines, other.bodyLines)
}

func requiredWaitMilliseconds(changedLines uint32) uint64 {
	return uint64(changedLines) * 1000 / changedLinesPerSecond
}

// formatSignatureInstant is an ISO 8601 instant with milliseconds, in UTC, ending in `Z`.
func formatSignatureInstant(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func parseSignatureInstant(at string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, fmt.Errorf("the signature recorded at %q is not an RFC 3339 instant, so the rate limit cannot measure from it: %v", at, err)
	}
	if expected := formatSignatureInstant(instant); expected != at {
		return time.Time{}, fmt.Errorf("signature instant expected canonical UTC milliseconds %q, found %q", expected, at)
	}
	return instant.UTC(), nil
}

// remainingWaitMilliseconds is how much of the wait is still to run when the hunk is signed at `at`, measured from the
// last pressed signature; zero and false when the wait is over or nothing was pressed yet.
func remainingWaitMilliseconds(signatures []hunkSignature, changedLines uint32, at time.Time) (uint64, bool, error) {
	for index := len(signatures) - 1; index >= 0; index-- {
		previous := signatures[index]
		if previous.Carried != nil && *previous.Carried {
			continue
		}
		previousAt, err := parseSignatureInstant(previous.At)
		if err != nil {
			return 0, false, err
		}
		elapsed := at.Sub(previousAt).Milliseconds()
		required := int64(requiredWaitMilliseconds(changedLines))
		if elapsed < required {
			return uint64(required - elapsed), true, nil
		}
		return 0, false, nil
	}
	return 0, false, nil
}

var sidecarLocks sync.Map

// withSidecarSerialized runs one read-modify-write of a sidecar with every other one of the same file held back.
func withSidecarSerialized[Value any](sidecarFile string, write func() (Value, error)) (Value, error) {
	lock, _ := sidecarLocks.LoadOrStore(sidecarFile, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()
	return write()
}

// readSignatureDocumentNamingAnyCandidate reads a sidecar whatever candidate it names. A missing sidecar is the empty
// document of `candidate`; a sidecar that does not read, or names another folder, is an error.
func readSignatureDocumentNamingAnyCandidate(sidecarFile, candidate string) (hunkSignatureDocument, error) {
	text, err := os.ReadFile(sidecarFile)
	if os.IsNotExist(err) {
		return hunkSignatureDocument{Candidate: candidate, Folder: rangeSignatureFolder, Signatures: []hunkSignature{}}, nil
	}
	if err != nil {
		return hunkSignatureDocument{}, fmt.Errorf("cannot read the signature sidecar %s: %v", sidecarFile, err)
	}
	notADocument := func(err error) error {
		return fmt.Errorf("the signature sidecar %s is not a signature document: %v; found %s", sidecarFile, err, text)
	}
	incompatible := func(field, found, reason string) error {
		return fmt.Errorf("%s candidate %s: %s found %s: %s", sidecarFile, candidate, field, found, reason)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(text, &raw); err != nil {
		return hunkSignatureDocument{}, notADocument(err)
	}
	if err := requireFields(text, []string{"candidate", "folder", "signatures"}); err != nil {
		return hunkSignatureDocument{}, notADocument(err)
	}
	if value, present := raw["carriedFrom"]; present {
		var carriedFrom string
		if json.Unmarshal(value, &carriedFrom) != nil || !validOid(carriedFrom) {
			return hunkSignatureDocument{}, incompatible("carriedFrom", string(value), "expected a full OID when present")
		}
	}
	var rawSignatures []map[string]json.RawMessage
	if err := json.Unmarshal(raw["signatures"], &rawSignatures); err != nil {
		return hunkSignatureDocument{}, notADocument(err)
	}
	for index, signature := range rawSignatures {
		encoded, _ := json.Marshal(signature)
		if err := requireFields(encoded, []string{"path", "hunk", "signer", "at"}); err != nil {
			return hunkSignatureDocument{}, notADocument(fmt.Errorf("signatures[%d]: %v", index, err))
		}
		carried, hasCarried := signature["carried"]
		signedAt, hasSignedAt := signature["signedAtCandidate"]
		var signedAtText string
		switch {
		case !hasCarried && !hasSignedAt:
		case hasCarried && hasSignedAt && string(carried) == "true" && json.Unmarshal(signedAt, &signedAtText) == nil && validOid(signedAtText):
		default:
			return hunkSignatureDocument{}, incompatible(fmt.Sprintf("signatures[%d].carried/signedAtCandidate", index), string(encoded),
				"expected both absent for a direct signature, or carried=true and signedAtCandidate=<full OID>")
		}
		var at string
		if json.Unmarshal(signature["at"], &at) == nil {
			if _, err := parseSignatureInstant(at); err != nil {
				return hunkSignatureDocument{}, incompatible(fmt.Sprintf("signatures[%d].at", index), string(signature["at"]), err.Error())
			}
		}
	}
	var document hunkSignatureDocument
	if err := decodeStrict(text, &document); err != nil {
		return hunkSignatureDocument{}, notADocument(err)
	}
	if document.Signatures == nil {
		document.Signatures = []hunkSignature{}
	}
	if document.Folder != rangeSignatureFolder {
		return hunkSignatureDocument{}, fmt.Errorf("the signature sidecar %s names the candidate %s and the folder %q, but it was read for the candidate %s and the folder %q",
			sidecarFile, document.Candidate, document.Folder, candidate, rangeSignatureFolder)
	}
	return document, nil
}

// readRangeSignatureDocument is the document that stands on the range's current MUST SIGN hunks.
func readRangeSignatureDocument(sidecarFile, candidate string, kind candidateKind, hunks []mustSignHunk) (hunkSignatureDocument, error) {
	document, err := readSignatureDocumentNamingAnyCandidate(sidecarFile, candidate)
	if err != nil {
		return document, err
	}
	if kind == candidateCommit {
		for index, signature := range document.Signatures {
			if signature.Content != nil {
				return document, fmt.Errorf("%s candidate %s: signatures[%d].content found present: only a working-tree signature is bound to hunk content; a commit's signature is bound to its hunk key",
					sidecarFile, candidate, index)
			}
		}
		if document.Candidate != candidate {
			return document, fmt.Errorf("the signature sidecar %s names the candidate %s and the folder %q, but it was read for the candidate %s and the folder %q",
				sidecarFile, document.Candidate, document.Folder, candidate, rangeSignatureFolder)
		}
		return document, validateSignatureHunks(sidecarFile, &document, hunks)
	}
	standing, err := standingWorkingTreeSignatures(sidecarFile, candidate, document.Signatures, hunks)
	if err != nil {
		return document, err
	}
	return hunkSignatureDocument{Candidate: candidate, Folder: document.Folder, Signatures: standing, CarriedFrom: document.CarriedFrom}, nil
}

func validateSignatureHunks(sidecarFile string, document *hunkSignatureDocument, hunks []mustSignHunk) error {
	for index, signature := range document.Signatures {
		known := slices.ContainsFunc(hunks, func(hunk mustSignHunk) bool { return hunk.path == signature.Path && hunk.key == signature.Hunk })
		if !known {
			return fmt.Errorf("%s candidate %s: signature[%d] path %q hunk %q is not a MUST SIGN hunk in this candidate",
				sidecarFile, document.Candidate, index, signature.Path, signature.Hunk)
		}
	}
	return nil
}

// standingWorkingTreeSignatures is the working-tree signatures that still stand on the current Diff, each re-keyed to
// the hunk it now covers: a stored signature stands on the current hunk with the same path and body at the same place
// among the identical hunks of its file.
func standingWorkingTreeSignatures(sidecarFile, candidate string, stored []hunkSignature, current []mustSignHunk) ([]hunkSignature, error) {
	type matchKey struct{ path, body string }
	keyOf := func(path string, body []string) matchKey {
		encoded, _ := json.Marshal(body)
		return matchKey{path, string(encoded)}
	}
	previous := map[matchKey][]*hunkSignature{}
	for index := range stored {
		signature := &stored[index]
		if signature.Content == nil {
			return nil, fmt.Errorf("%s candidate %s: signatures[%d].content found absent: a working-tree signature records the body and occurrence of the hunk it signed",
				sidecarFile, candidate, index)
		}
		key := keyOf(signature.Path, signature.Content.Body)
		place := int(signature.Content.Occurrence)
		for len(previous[key]) <= place {
			previous[key] = append(previous[key], nil)
		}
		if previous[key][place] != nil {
			return nil, fmt.Errorf("%s candidate %s: signatures[%d].content.occurrence found %d: another signature of %q already signs this occurrence of the same body",
				sidecarFile, candidate, index, signature.Content.Occurrence, signature.Path)
		}
		previous[key][place] = signature
	}
	standing := []hunkSignature{}
	for index := range current {
		hunk := &current[index]
		key := keyOf(hunk.path, hunk.bodyLines)
		queue := previous[key]
		if len(queue) == 0 {
			continue
		}
		signature := queue[0]
		previous[key] = queue[1:]
		if signature == nil {
			continue
		}
		content := *signature.Content
		standing = append(standing, hunkSignature{Path: hunk.path, Hunk: hunk.key, Signer: signature.Signer, At: signature.At, Content: &content})
	}
	return standing, nil
}

// workingTreeSignatureContent is the content a working-tree signature pressed on `hunk` is bound to.
func workingTreeSignatureContent(hunks []mustSignHunk, hunk *mustSignHunk) *hunkSignatureContent {
	var occurrence uint32
	for index := range hunks {
		if hunks[index].ordinal == hunk.ordinal {
			break
		}
		if hunks[index].sameContent(hunk) {
			occurrence++
		}
	}
	return &hunkSignatureContent{Body: slices.Clone(hunk.bodyLines), Occurrence: occurrence}
}

var temporarySidecarCounter atomic.Uint64

// writeSignatureDocument writes the sidecar as pretty JSON with a trailing newline, into a uniquely named temporary file
// beside it that is then renamed over it, so a reader never sees a half-written document.
func writeSignatureDocument(sidecarFile string, document *hunkSignatureDocument) error {
	unwritable := func(err error) error {
		return fmt.Errorf("cannot write the signature sidecar %s: %v", sidecarFile, err)
	}
	directory := filepath.Dir(sidecarFile)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return unwritable(err)
	}
	text, err := marshalIndented(document)
	if err != nil {
		panic(err)
	}
	temporary := filepath.Join(directory, fmt.Sprintf("%s.%d-%d.tmp", filepath.Base(sidecarFile), os.Getpid(), temporarySidecarCounter.Add(1)-1))
	err = os.WriteFile(temporary, append(text, '\n'), 0o644)
	if err == nil {
		err = os.Rename(temporary, sidecarFile)
	}
	if err != nil {
		if cleanup := os.Remove(temporary); cleanup != nil && !os.IsNotExist(cleanup) {
			return fmt.Errorf("cannot write %s: %v; cannot clean temporary file %s: %v", sidecarFile, err, temporary, cleanup)
		}
		return unwritable(err)
	}
	return nil
}

// marshalIndented is the two-space indented JSON of a value, with no HTML escaping.
func marshalIndented(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// readRangeSignatures is the whole signature state of the range.
func readRangeSignatures(sidecarFile, candidate string, kind candidateKind, diff *rangeDiff) (*hunkSignatures, error) {
	hunks := mustSignHunksInDisplayOrder(diff)
	document, err := withSidecarSerialized(sidecarFile, func() (hunkSignatureDocument, error) {
		return readRangeSignatureDocument(sidecarFile, candidate, kind, hunks)
	})
	if err != nil {
		return nil, err
	}
	if err := validateSignatureHunks(sidecarFile, &document, hunks); err != nil {
		return nil, err
	}
	first := map[[2]string]*hunkSignature{}
	for index := range document.Signatures {
		signature := &document.Signatures[index]
		key := [2]string{signature.Path, signature.Hunk}
		if _, known := first[key]; !known {
			first[key] = signature
		}
	}
	signatures := &hunkSignatures{document: document}
	perFile := map[string][2]uint32{}
	for _, hunk := range hunks {
		state := mustSignHunkSignatureState{Path: hunk.path, Hunk: hunk.key, Range: hunk.header, Ordinal: hunk.ordinal,
			Total: uint32(len(hunks)), ChangedLines: hunk.changedLines}
		counts := perFile[hunk.path]
		counts[0]++
		if signature := first[[2]string{hunk.path, hunk.key}]; signature != nil {
			copied := *signature
			state.Signature = &copied
			signatures.progress.signedHunks++
			counts[1]++
		}
		perFile[hunk.path] = counts
		signatures.hunks = append(signatures.hunks, state)
	}
	signatures.progress.totalHunks = uint32(len(hunks))
	for _, counts := range perFile {
		if counts[0] > 0 && counts[0] == counts[1] {
			signatures.progress.signedFiles++
		}
	}
	signatures.progress.totalFiles = diff.statistics.mustSignFiles
	return signatures, nil
}

// signRangeMustSignHunk records one press of the Sign button under the rate limit.
func signRangeMustSignHunk(sidecarFile, candidate string, kind candidateKind, diff *rangeDiff, path, key, signer string, at time.Time) (hunkSignatureWrite, error) {
	signer = strings.TrimSpace(signer)
	if signer == "" {
		return hunkSignatureWrite{}, fmt.Errorf("the signer name is blank")
	}
	hunks := mustSignHunksInDisplayOrder(diff)
	index := slices.IndexFunc(hunks, func(hunk mustSignHunk) bool { return hunk.path == path && hunk.key == key })
	if index < 0 {
		return hunkSignatureWrite{}, fmt.Errorf("the hunk %q of %q is not a MUST SIGN hunk of the Diff of candidate %s, so it cannot be signed", key, path, candidate)
	}
	signed := &hunks[index]
	write := hunkSignatureWrite{Path: path, Hunk: key, ChangedLines: signed.changedLines, RequiredWaitMilliseconds: requiredWaitMilliseconds(signed.changedLines)}
	return withSidecarSerialized(sidecarFile, func() (hunkSignatureWrite, error) {
		document, err := readRangeSignatureDocument(sidecarFile, candidate, kind, hunks)
		if err != nil {
			return write, err
		}
		if slices.ContainsFunc(document.Signatures, func(signature hunkSignature) bool { return signature.Path == path && signature.Hunk == key }) {
			write.Outcome = "already_signed"
			return write, nil
		}
		remaining, waiting, err := remainingWaitMilliseconds(document.Signatures, signed.changedLines, at)
		if err != nil {
			return write, err
		}
		if waiting {
			message := signsTooFastMessage
			write.Outcome, write.RemainingMilliseconds, write.Message = "refused_as_too_fast", &remaining, &message
			return write, nil
		}
		signature := hunkSignature{Path: path, Hunk: key, Signer: signer, At: formatSignatureInstant(at)}
		if kind == candidateWorkingTree {
			signature.Content = workingTreeSignatureContent(hunks, signed)
		}
		document.Signatures = append(document.Signatures, signature)
		if err := writeSignatureDocument(sidecarFile, &document); err != nil {
			return write, err
		}
		write.Outcome = "signed"
		return write, nil
	})
}

// signRangeUnsignedHunksUnderDirectory signs every unsigned MUST SIGN hunk under `directory` at once.
func signRangeUnsignedHunksUnderDirectory(sidecarFile, candidate string, kind candidateKind, diff *rangeDiff, directory, signer string, at time.Time) error {
	signer = strings.TrimSpace(signer)
	if signer == "" {
		return fmt.Errorf("the signer name is blank")
	}
	hunks := mustSignHunksInDisplayOrder(diff)
	prefix := directory + "/"
	_, err := withSidecarSerialized(sidecarFile, func() (struct{}, error) {
		document, err := readRangeSignatureDocument(sidecarFile, candidate, kind, hunks)
		if err != nil {
			return struct{}{}, err
		}
		existing := map[[2]string]bool{}
		for _, signature := range document.Signatures {
			existing[[2]string{signature.Path, signature.Hunk}] = true
		}
		before := len(document.Signatures)
		for index := range hunks {
			hunk := &hunks[index]
			if !strings.HasPrefix(hunk.path, prefix) || existing[[2]string{hunk.path, hunk.key}] {
				continue
			}
			signature := hunkSignature{Path: hunk.path, Hunk: hunk.key, Signer: signer, At: formatSignatureInstant(at)}
			if kind == candidateWorkingTree {
				signature.Content = workingTreeSignatureContent(hunks, hunk)
			}
			document.Signatures = append(document.Signatures, signature)
		}
		if len(document.Signatures) == before {
			return struct{}{}, nil
		}
		return struct{}{}, writeSignatureDocument(sidecarFile, &document)
	})
	return err
}
