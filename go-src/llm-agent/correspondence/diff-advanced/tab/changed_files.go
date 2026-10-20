package tab

// Every changed path of the range, read from Git, classified, laid out as the Diff tree, with the MUST SIGN hunks
// numbered in tree order.

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// diffContextLines is the context of the reviewer Diff and of the MUST SIGN hunks.
const diffContextLines = 3

// absentDiffSidePath is the path a side of the Diff carries when the file is absent there.
const absentDiffSidePath = "/dev/null"

type changeKind int

const (
	changeAdded changeKind = iota
	changeDeleted
	changeModified
	changeRenamed
	changeCopied
	changeTypeChanged
)

func (kind changeKind) wireWord() string {
	return [...]string{"added", "deleted", "modified", "renamed", "copied", "type_changed"}[kind]
}

// changeWords is the style class and the word of a change kind.
func (kind changeKind) changeWords() (string, string) {
	switch kind {
	case changeAdded:
		return "add", "Added"
	case changeDeleted:
		return "delete", "Deleted"
	case changeModified:
		return "modify", "Modified"
	case changeRenamed:
		return "rename", "Renamed"
	case changeCopied:
		return "copy", "Copied"
	}
	return "type-change", "Type changed"
}

// hunkFields is one hunk of one changed file.
type hunkFields struct {
	// number is the position of the hunk inside its file, counting from one.
	number uint32
	// mustSignOrdinal is the position among every MUST SIGN hunk of the Diff in tree order, zero when not MUST SIGN.
	mustSignOrdinal    uint32
	header             string
	headerLine         string
	oldStart           uint32
	oldLines           uint32
	newStart           uint32
	newLines           uint32
	lines              []diffLine
	commentUnifiedText string
}

type diffFile struct {
	path             string
	oldPath          string
	newPath          string
	changeKind       changeKind
	classification   fileClassification
	mustSignReason   string
	isBinary         bool
	additions        uint32
	deletions        uint32
	oldEndingNewline bool
	newEndingNewline bool
	hunks            []hunkFields
}

type unnumberedFile = diffFile

func (file *diffFile) mustSign() bool { return file.classification.kind == kindMustSign }

type treeStatistics struct {
	files     uint32
	hunks     uint32
	additions uint32
	deletions uint32
}

func sumStatistics(parts ...treeStatistics) treeStatistics {
	var total treeStatistics
	for _, part := range parts {
		total.files += part.files
		total.hunks += part.hunks
		total.additions += part.additions
		total.deletions += part.deletions
	}
	return total
}

type treeNode struct {
	isDirectory bool
	// path is the repository path of the deepest directory of a compacted chain, or the file's listed path.
	path string
	// label is the chain's names joined by `/`, or the file's name.
	label      string
	children   []treeNode
	statistics treeStatistics
	fileIndex  int
}

type subclassGroup struct {
	subclass   fileSubclass
	nodes      []treeNode
	statistics treeStatistics
}

type classGroup struct {
	class          fileClass
	subclassGroups []subclassGroup
	statistics     treeStatistics
}

type kindGroup struct {
	kind        fileKind
	classGroups []classGroup
	statistics  treeStatistics
}

type diffStatistics struct {
	files         uint32
	additions     uint32
	deletions     uint32
	mustSignFiles uint32
	mustSignHunks uint32
}

// rangeDiff is the whole Diff of the range: its two commits, the changed files in Git's order, the tree and the counts.
type rangeDiff struct {
	base       string
	candidate  string
	files      []diffFile
	tree       []kindGroup
	statistics diffStatistics
}

// computeRangeIndex is the index of `base..candidate` under the instance's MUST SIGN declarations: every file's hunk
// headers and the bodies of the MUST SIGN files' hunks.
func computeRangeIndex(repository *GitRepository, base, candidate string, policy *mustSignPolicy, additionsFile string) (*rangeDiff, error) {
	files, err := readFiles(repository, base, candidate, diffSelection{}, policy, true)
	if err != nil {
		return nil, err
	}
	problems, err := additionProblems(policy, files, func(path string) (int, error) {
		blob, err := repository.ShowBlob(candidate, path)
		if err != nil {
			return 0, err
		}
		return len(strLines(string(blob))), nil
	})
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s at %s: the MUST SIGN additions are invalid: %s", additionsFile, candidate, strings.Join(problems, "; "))
	}
	diff := &rangeDiff{base: base, candidate: candidate, files: files}
	diff.tree = buildDiffTree(files)
	var ordinal uint32
	for _, index := range mustSignFilesInDisplayOrder(diff.tree) {
		file := &files[index]
		for hunkIndex := range file.hunks {
			hunk := &file.hunks[hunkIndex]
			if policy.hunkIsMustSign(file.path, hunk.newStart, hunk.newLines) {
				ordinal++
				hunk.mustSignOrdinal = ordinal
			}
		}
	}
	diff.statistics.files = uint32(len(files))
	diff.statistics.mustSignHunks = ordinal
	for _, file := range files {
		diff.statistics.additions += file.additions
		diff.statistics.deletions += file.deletions
		if file.mustSign() {
			diff.statistics.mustSignFiles++
		}
	}
	return diff, nil
}

func readFiles(repository *GitRepository, base, candidate string, selection diffSelection, policy *mustSignPolicy, indexOnly bool) ([]diffFile, error) {
	nameStatus, err := repository.diffNameStatus(base, candidate, selection)
	if err != nil {
		return nil, err
	}
	lines, err := repository.unifiedDiffLines(base, candidate, selection, diffContextLines)
	if err != nil {
		return nil, err
	}
	sections, err := parseUnifiedPatchLines(lines, func(index int) bool {
		return !indexOnly || index < len(nameStatus) && classify(nameStatus[index].path, policy).kind == kindMustSign
	})
	if err != nil {
		return nil, err
	}
	if len(sections) != len(nameStatus) {
		return nil, fmt.Errorf("`git diff --name-status` reports %d changed files but the patch carries %d file sections",
			len(nameStatus), len(sections))
	}
	files := make([]diffFile, 0, len(nameStatus))
	for index, entry := range nameStatus {
		file, err := buildFile(entry, &sections[index], policy)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func buildFile(entry nameStatusEntry, section *patchFileSection, policy *mustSignPolicy) (diffFile, error) {
	var kind changeKind
	switch entry.status[0] {
	case 'A':
		kind = changeAdded
	case 'D':
		kind = changeDeleted
	case 'M':
		kind = changeModified
	case 'R':
		kind = changeRenamed
	case 'C':
		kind = changeCopied
	case 'T':
		kind = changeTypeChanged
	default:
		return diffFile{}, fmt.Errorf("`git diff --name-status` reports the status %q for %q, which this server does not read (it reads A, D, M, R, C and T)",
			entry.status, entry.path)
	}
	// A file is listed and classified under its new path, except a deletion, whose only path is the old one.
	path := entry.path
	oldPath, newPath := path, path
	switch kind {
	case changeAdded:
		oldPath = absentDiffSidePath
	case changeRenamed, changeCopied:
		oldPath = entry.sourcePath
	case changeDeleted:
		newPath = absentDiffSidePath
	}
	for _, side := range []struct {
		name, nameStatusPath, patchPath string
		present                         bool
	}{{"old", oldPath, section.oldHeaderPath, section.hasOldHeaderPath}, {"new", newPath, section.newHeaderPath, section.hasNewHeaderPath}} {
		if side.present && side.patchPath != side.nameStatusPath {
			return diffFile{}, fmt.Errorf("`git diff --name-status` reports the %s path %q but the patch header names %q",
				side.name, side.nameStatusPath, side.patchPath)
		}
	}
	file := diffFile{
		path: path, oldPath: oldPath, newPath: newPath, changeKind: kind, classification: classify(path, policy),
		isBinary: section.isBinary, oldEndingNewline: section.oldEndingNewline, newEndingNewline: section.newEndingNewline,
	}
	if addition := policy.addition(path); addition != nil {
		file.mustSignReason = addition.Reason
	}
	for index := range section.hunks {
		hunk := &section.hunks[index]
		file.additions += hunk.additions
		file.deletions += hunk.deletions
		file.hunks = append(file.hunks, hunkFields{
			number: uint32(index + 1), header: hunk.canonicalHeader(), headerLine: hunk.headerLine,
			oldStart: hunk.oldStart, oldLines: hunk.oldLines, newStart: hunk.newStart, newLines: hunk.newLines,
			lines: hunk.lines, commentUnifiedText: commentUnifiedText(hunk.lines),
		})
	}
	return file, nil
}

// commentUnifiedText is the unified text of a hunk for a comment, with at most eighty source lines.
func commentUnifiedText(lines []diffLine) string {
	const shownLines = 80
	var shown []string
	for index, line := range lines {
		if index == shownLines {
			break
		}
		marker := map[lineKind]string{lineContext: " ", lineInsertion: "+", lineDeletion: "-"}[line.kind]
		shown = append(shown, marker+line.text)
	}
	if len(lines) > shownLines {
		shown = append(shown, fmt.Sprintf("… (%d more lines)", len(lines)-shownLines))
	}
	return strings.Join(shown, "\n")
}

// loadFile fetches the opened file's hunk bodies; the index keeps every header and navigation position.
func loadFile(repository *GitRepository, diff *rangeDiff, file *diffFile) (*diffFile, error) {
	selection := diffSelection{path: file.path}
	if file.changeKind == changeRenamed || file.changeKind == changeCopied {
		selection = diffSelection{path: file.newPath, sourcePath: file.oldPath}
	}
	files, err := readFiles(repository, diff.base, diff.candidate, selection, &mustSignPolicy{}, false)
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(files, func(loaded diffFile) bool { return loaded.path == file.path })
	if index < 0 {
		return nil, fmt.Errorf("%s at %s: opened file absent from diff", file.path, diff.candidate)
	}
	loaded := files[index]
	for hunkIndex := range loaded.hunks {
		if hunkIndex < len(file.hunks) {
			loaded.hunks[hunkIndex].mustSignOrdinal = file.hunks[hunkIndex].mustSignOrdinal
		}
	}
	loaded.classification = file.classification
	loaded.mustSignReason = file.mustSignReason
	return &loaded, nil
}

// buildDiffTree lays the files out by kind, class and subclass: kinds in display order and a kind no file has left
// out; classes in declaration order and a class no file of the kind has left out; every subclass the kind exposes for a
// present class, the empty ones included; per subclass a directory tree.
func buildDiffTree(files []diffFile) []kindGroup {
	var groups []kindGroup
	for _, kind := range kindDisplayOrder {
		var inKind []int
		for index := range files {
			if files[index].classification.kind == kind {
				inKind = append(inKind, index)
			}
		}
		if len(inKind) == 0 {
			continue
		}
		group := kindGroup{kind: kind}
		for _, entry := range classDisplayOrder {
			var inClass []int
			for _, index := range inKind {
				if files[index].classification.class == entry.class {
					inClass = append(inClass, index)
				}
			}
			if len(inClass) == 0 {
				continue
			}
			class := classGroup{class: entry.class}
			for _, subclass := range subclassesFor(kind, entry.class) {
				var inSubclass []int
				var statistics []treeStatistics
				for _, index := range inClass {
					if files[index].classification.subclass == subclass {
						inSubclass = append(inSubclass, index)
						statistics = append(statistics, fileStatistics(&files[index]))
					}
				}
				class.subclassGroups = append(class.subclassGroups, subclassGroup{
					subclass: subclass, nodes: buildDirectoryTree(files, inSubclass, ""), statistics: sumStatistics(statistics...),
				})
				class.statistics = sumStatistics(class.statistics, class.subclassGroups[len(class.subclassGroups)-1].statistics)
			}
			group.classGroups = append(group.classGroups, class)
			group.statistics = sumStatistics(group.statistics, class.statistics)
		}
		groups = append(groups, group)
	}
	return groups
}

func fileStatistics(file *diffFile) treeStatistics {
	return treeStatistics{files: 1, hunks: uint32(len(file.hunks)), additions: file.additions, deletions: file.deletions}
}

func buildDirectoryTree(files []diffFile, indices []int, prefix string) []treeNode {
	type directory struct {
		name    string
		indices []int
	}
	var directories []directory
	positions := map[string]int{}
	var nodes []treeNode
	for _, index := range indices {
		relative := files[index].path[len(prefix):]
		slash := strings.IndexByte(relative, '/')
		if slash < 0 {
			nodes = append(nodes, treeNode{path: files[index].path, label: relative, fileIndex: index})
			continue
		}
		name := relative[:slash]
		if position, known := positions[name]; known {
			directories[position].indices = append(directories[position].indices, index)
		} else {
			positions[name] = len(directories)
			directories = append(directories, directory{name: name, indices: []int{index}})
		}
	}
	for _, entry := range directories {
		path := prefix + entry.name
		label := entry.name
		children := buildDirectoryTree(files, entry.indices, path+"/")
		// A single-directory chain is compacted, as an Explorer with compact folders does.
		if len(children) == 1 && children[0].isDirectory {
			path, label, children = children[0].path, label+"/"+children[0].label, children[0].children
		}
		var statistics []treeStatistics
		for _, index := range entry.indices {
			statistics = append(statistics, fileStatistics(&files[index]))
		}
		nodes = append(nodes, treeNode{isDirectory: true, path: path, label: label, children: children, statistics: sumStatistics(statistics...)})
	}
	slices.SortStableFunc(nodes, func(left, right treeNode) int {
		if left.isDirectory != right.isDirectory {
			if left.isDirectory {
				return -1
			}
			return 1
		}
		return compareLabels(left.label, right.label)
	})
	return nodes
}

// mustSignFilesInDisplayOrder is the indices of the MUST SIGN files in tree order, the order their hunks are
// numbered in.
func mustSignFilesInDisplayOrder(tree []kindGroup) []int {
	var order []int
	for _, group := range tree {
		if group.kind != kindMustSign {
			continue
		}
		for _, class := range group.classGroups {
			for _, subclass := range class.subclassGroups {
				order = appendFileIndices(order, subclass.nodes)
			}
		}
	}
	return order
}

func appendFileIndices(order []int, nodes []treeNode) []int {
	for _, node := range nodes {
		if node.isDirectory {
			order = appendFileIndices(order, node.children)
		} else {
			order = append(order, node.fileIndex)
		}
	}
	return order
}

// filesInDisplayOrder is the changed files in the order the Diff tree lists them.
func (diff *rangeDiff) filesInDisplayOrder() []*diffFile {
	var order []int
	for _, group := range diff.tree {
		for _, class := range group.classGroups {
			for _, subclass := range class.subclassGroups {
				order = appendFileIndices(order, subclass.nodes)
			}
		}
	}
	files := make([]*diffFile, 0, len(order))
	for _, index := range order {
		files = append(files, &diff.files[index])
	}
	return files
}

func (diff *rangeDiff) file(path string) *diffFile {
	for index := range diff.files {
		if diff.files[index].path == path {
			return &diff.files[index]
		}
	}
	return nil
}

// compareLabels is `label.localeCompare(other, undefined, { numeric: true, sensitivity: 'base' })` for the labels a
// repository path produces: a run of digits compares by value, every other character compares case-folded, and a
// prefix sorts before the longer label.
func compareLabels(left, right string) int {
	leftCharacters, rightCharacters := []rune(left), []rune(right)
	l, r := 0, 0
	isDigit := func(character rune) bool { return character >= '0' && character <= '9' }
	for {
		switch {
		case l == len(leftCharacters) && r == len(rightCharacters):
			return 0
		case l == len(leftCharacters):
			return -1
		case r == len(rightCharacters):
			return 1
		}
		if isDigit(leftCharacters[l]) && isDigit(rightCharacters[r]) {
			leftStart, rightStart := l, r
			for l < len(leftCharacters) && isDigit(leftCharacters[l]) {
				l++
			}
			for r < len(rightCharacters) && isDigit(rightCharacters[r]) {
				r++
			}
			leftDigits := strings.TrimLeft(string(leftCharacters[leftStart:l]), "0")
			rightDigits := strings.TrimLeft(string(rightCharacters[rightStart:r]), "0")
			if ordering := cmp.Compare(len(leftDigits), len(rightDigits)); ordering != 0 {
				return ordering
			}
			if ordering := strings.Compare(leftDigits, rightDigits); ordering != 0 {
				return ordering
			}
			continue
		}
		ordering := slices.Compare(caseFolded(leftCharacters[l]), caseFolded(rightCharacters[r]))
		l++
		r++
		if ordering != 0 {
			return ordering
		}
	}
}

func caseFolded(character rune) []rune {
	if character < unicode.MaxASCII {
		return []rune{unicode.ToLower(character)}
	}
	return []rune(strings.ToLower(string(character)))
}
