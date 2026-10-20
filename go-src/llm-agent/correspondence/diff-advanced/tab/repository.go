package tab

// The repository reader of the diff/advanced range: read-only Git queries with a pinned configuration and environment,
// and the working tree written as a commit object.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// GitRepository is a Git checkout the range is read from; every query runs `git -C <root>`.
type GitRepository struct {
	root string
}

func NewGitRepository(root string) *GitRepository { return &GitRepository{root: root} }

func (repository *GitRepository) Root() string { return repository.root }

var gitConfiguration = []string{
	"-c", "core.quotePath=true",
	"-c", "core.abbrev=7",
	"-c", "core.attributesFile=/dev/null",
	"-c", "diff.suppressBlankEmpty=false",
	"-c", "diff.renameLimit=1000",
}

// queryCommand is `git -C <root> <configuration> <arguments>` with every inherited GIT_ variable removed and the
// environment pinned; `source` names the commit whose attributes apply.
func (repository *GitRepository) queryCommand(arguments []string, source string, extraEnvironment []string) *exec.Cmd {
	all := append([]string{"-C", repository.root}, gitConfiguration...)
	all = append(all, arguments...)
	command := exec.Command("git", all...)
	var environment []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GIT_") || strings.HasPrefix(entry, "LC_ALL=") {
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment, "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1")
	if source != "" {
		environment = append(environment, "GIT_ATTR_SOURCE="+source)
	}
	command.Env = append(environment, extraEnvironment...)
	return command
}

func (repository *GitRepository) run(arguments []string, source string, extraEnvironment []string) ([]byte, error) {
	command := repository.queryCommand(arguments, source, extraEnvironment)
	var standardOutput, standardError bytes.Buffer
	command.Stdout = &standardOutput
	command.Stderr = &standardError
	if err := command.Run(); err != nil {
		if _, exited := err.(*exec.ExitError); !exited {
			return nil, fmt.Errorf("git -C %s %s: cannot start: %v", repository.root, strings.Join(arguments, " "), err)
		}
		return nil, fmt.Errorf("`git -C %s %s` failed: %v: %s", repository.root, strings.Join(arguments, " "), err,
			strings.TrimSpace(standardError.String()))
	}
	return standardOutput.Bytes(), nil
}

func (repository *GitRepository) queryText(arguments []string, source string) (string, error) {
	output, err := repository.run(arguments, source, nil)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(output) {
		return "", fmt.Errorf("git -C %s %s: non-UTF8 stdout", repository.root, strings.Join(arguments, " "))
	}
	return string(output), nil
}

// queryLines is the command's output split into lines as a line reader reads them: `\n` or `\r\n` ends a line, and a
// line that is not UTF-8 is an error.
func (repository *GitRepository) queryLines(arguments []string, source string) ([]string, error) {
	output, err := repository.run(arguments, source, nil)
	if err != nil {
		return nil, err
	}
	var lines []string
	for len(output) > 0 {
		index := bytes.IndexByte(output, '\n')
		var line []byte
		if index < 0 {
			line, output = output, nil
		} else {
			line, output = output[:index], output[index+1:]
			line = bytes.TrimSuffix(line, []byte("\r"))
		}
		if !utf8.Valid(line) {
			return nil, fmt.Errorf("git -C %s %s: stream did not contain valid UTF-8", repository.root, strings.Join(arguments, " "))
		}
		lines = append(lines, string(line))
	}
	return lines, nil
}

func validOid(text string) bool {
	if len(text) < 40 || len(text) > 64 {
		return false
	}
	for _, character := range []byte(text) {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func parseOid(text string) (string, error) {
	if !validOid(text) {
		return "", fmt.Errorf("invalid object id: %s", text)
	}
	return text, nil
}

func validatePath(path string) error {
	if strings.ContainsAny(path, "\\\x00\r\n") {
		return fmt.Errorf("not a repository-relative path: %q", path)
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("not a repository-relative path: %q", path)
		}
	}
	return nil
}

// RevParseCommit resolves a revision to the commit it names.
func (repository *GitRepository) RevParseCommit(revision string) (string, error) {
	output, err := repository.queryText([]string{"rev-parse", "--verify", "--end-of-options", revision + "^{commit}"}, "")
	if err != nil {
		return "", err
	}
	return parseOid(strings.TrimSpace(output))
}

func (repository *GitRepository) PathExistsAtCommit(commit, path string) (bool, error) {
	if err := validatePath(path); err != nil {
		return false, err
	}
	if _, err := repository.RevParseCommit(commit); err != nil {
		return false, err
	}
	output, err := repository.queryText([]string{"ls-tree", "--name-only", "-z", commit, "--", path}, "")
	if err != nil {
		return false, err
	}
	for _, found := range strings.Split(output, "\x00") {
		if found == path {
			return true, nil
		}
	}
	return false, nil
}

// ShowBlob is the bytes of `path` at `commit`, which must hold it.
func (repository *GitRepository) ShowBlob(commit, path string) ([]byte, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	listing, err := repository.queryText([]string{"ls-tree", "-r", "--name-only", "-z", commit, "--", path}, "")
	if err != nil {
		return nil, err
	}
	found := false
	for _, listed := range strings.Split(listing, "\x00") {
		if listed == path {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("`%s` is not in the tree of commit %s", path, commit)
	}
	return repository.run([]string{"cat-file", "blob", commit + ":" + path}, "", nil)
}

// renameOptions are `--find-renames=50%` with `-l0`, the shared reviewer Diff setting.
var renameOptions = []string{"--find-renames=50%", "-l0"}

var pinnedDiffOptions = []string{
	"--no-ext-diff", "--no-textconv", "--no-color", "--no-relative", "--diff-algorithm=myers", "--indent-heuristic",
	"--submodule=short", "--ignore-submodules=none", "-O/dev/null",
}

var pinnedPatchPresentationOptions = []string{
	"--src-prefix=a/", "--dst-prefix=b/", "--output-indicator-new=+", "--output-indicator-old=-",
	"--output-indicator-context= ",
}

// diffSelection is what one diff covers: the whole range when `path` is empty, or one file and, for a rename or copy,
// its source path.
type diffSelection struct {
	path       string
	sourcePath string
}

func (selection diffSelection) pathspecs() []string {
	if selection.path == "" {
		return nil
	}
	var pathspecs []string
	if selection.sourcePath != "" {
		pathspecs = append(pathspecs, ":(top,literal)"+selection.sourcePath)
	}
	return append(pathspecs, ":(top,literal)"+selection.path)
}

func (repository *GitRepository) unifiedDiffLines(base, candidate string, selection diffSelection, contextLines int) ([]string, error) {
	arguments := []string{"diff"}
	arguments = append(arguments, pinnedDiffOptions...)
	arguments = append(arguments, renameOptions...)
	arguments = append(arguments, "--patch", fmt.Sprintf("--unified=%d", contextLines), "--inter-hunk-context=0")
	arguments = append(arguments, pinnedPatchPresentationOptions...)
	arguments = append(arguments, base, candidate, "--")
	arguments = append(arguments, selection.pathspecs()...)
	return repository.queryLines(arguments, candidate)
}

// nameStatusEntry is one record of `git diff --name-status -z`.
type nameStatusEntry struct {
	status     string
	path       string
	sourcePath string
	hasSource  bool
}

func (repository *GitRepository) diffNameStatus(base, candidate string, selection diffSelection) ([]nameStatusEntry, error) {
	arguments := []string{"diff"}
	arguments = append(arguments, pinnedDiffOptions...)
	arguments = append(arguments, renameOptions...)
	arguments = append(arguments, "--name-status", "-z", base, candidate, "--")
	arguments = append(arguments, selection.pathspecs()...)
	commandLine := strings.Join(arguments, " ")
	output, err := repository.queryText(arguments, candidate)
	if err != nil {
		return nil, err
	}
	var fields []string
	for _, field := range strings.Split(output, "\x00") {
		if field != "" {
			fields = append(fields, field)
		}
	}
	var entries []nameStatusEntry
	for index := 0; index < len(fields); {
		status := fields[index]
		index++
		namesASource := strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")
		if index >= len(fields) {
			return nil, fmt.Errorf("%s: malformed name-status %q", commandLine, status)
		}
		firstPath := fields[index]
		index++
		if namesASource {
			if index >= len(fields) {
				return nil, fmt.Errorf("%s: malformed name-status %q", commandLine, status+"\x00"+firstPath)
			}
			entries = append(entries, nameStatusEntry{status: status, path: fields[index], sourcePath: firstPath, hasSource: true})
			index++
		} else {
			entries = append(entries, nameStatusEntry{status: status, path: firstPath})
		}
	}
	return entries, nil
}

var snapshotIdentityEnvironment = []string{
	"GIT_AUTHOR_NAME=working tree", "GIT_AUTHOR_EMAIL=working-tree@localhost", "GIT_AUTHOR_DATE=@0 +0000",
	"GIT_COMMITTER_NAME=working tree", "GIT_COMMITTER_EMAIL=working-tree@localhost", "GIT_COMMITTER_DATE=@0 +0000",
}

var temporaryIndexCounter atomic.Uint64

// WorkingTreeSnapshotCommit writes the working tree, that is tracked modifications and deletions together with
// untracked files that are not ignored, as a commit whose only parent is `HEAD`. Only objects are written: no ref
// moves, and the index and the files stay as they are, since the snapshot stages into a temporary copy of the index.
func (repository *GitRepository) WorkingTreeSnapshotCommit() (string, error) {
	indexText, err := repository.queryText([]string{"rev-parse", "--git-path", "index"}, "")
	if err != nil {
		return "", err
	}
	index := strings.TrimSpace(indexText)
	if !filepath.IsAbs(index) {
		index = filepath.Join(repository.root, index)
	}
	temporary := filepath.Join(os.TempDir(), fmt.Sprintf("proposal-working-tree-snapshot-%d-%d.index", os.Getpid(),
		temporaryIndexCounter.Add(1)-1))
	contents, err := os.ReadFile(index)
	if err != nil {
		return "", fmt.Errorf("cannot copy the index %s to %s: %v", index, temporary, err)
	}
	if err := os.WriteFile(temporary, contents, 0o644); err != nil {
		return "", fmt.Errorf("cannot copy the index %s to %s: %v", index, temporary, err)
	}
	defer func() {
		if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
			panic(fmt.Sprintf("cannot remove the temporary index %s: %v", temporary, err))
		}
	}()
	withIndex := func(arguments []string, environment []string) (string, error) {
		output, err := repository.run(arguments, "", append([]string{"GIT_INDEX_FILE=" + temporary}, environment...))
		if err != nil {
			return "", err
		}
		if !utf8.Valid(output) {
			return "", fmt.Errorf("git %s: non-UTF8 stdout", strings.Join(arguments, " "))
		}
		return string(output), nil
	}
	if _, err := withIndex([]string{"add", "--all", "--", ":/"}, nil); err != nil {
		return "", err
	}
	tree, err := withIndex([]string{"write-tree"}, nil)
	if err != nil {
		return "", err
	}
	commit, err := withIndex([]string{"commit-tree", "--no-gpg-sign", "-p", "HEAD", "-m", "working tree", strings.TrimSpace(tree)},
		snapshotIdentityEnvironment)
	if err != nil {
		return "", err
	}
	return parseOid(strings.TrimSpace(commit))
}
