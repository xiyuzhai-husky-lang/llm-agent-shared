package tab

// The Diff tree as the page prints it: kind, class and subclass headings and each subclass's directories and files with
// the counts every row shows. The view carries the headings and each subclass's top rows; a directory's rows are
// answered when the page opens it.

import (
	"fmt"
	"strings"
)

type diffPosition struct {
	Path string `json:"path"`
	Hunk string `json:"hunk"`
}

type statsView struct {
	FilesLabel     *string `json:"files_label"`
	HunksLabel     string  `json:"hunks_label"`
	HunksTitle     string  `json:"hunks_title"`
	HunksAriaLabel *string `json:"hunks_aria_label"`
	AdditionsLabel *string `json:"additions_label"`
	DeletionsLabel *string `json:"deletions_label"`
	AriaLabel      string  `json:"aria_label"`
}

type groupHeadingView struct {
	Label         string        `json:"label"`
	AriaLabel     string        `json:"aria_label"`
	Empty         bool          `json:"empty"`
	FirstPosition *diffPosition `json:"first_position"`
	Stats         statsView     `json:"stats"`
}

type kindGroupView struct {
	Kind    string           `json:"kind"`
	Heading groupHeadingView `json:"heading"`
	Classes []classGroupView `json:"classes"`
}

type classGroupView struct {
	Key        string              `json:"key"`
	Heading    groupHeadingView    `json:"heading"`
	Subclasses []subclassGroupView `json:"subclasses"`
}

type subclassGroupView struct {
	Key     string           `json:"key"`
	Heading groupHeadingView `json:"heading"`
	Nodes   []treeNodeView   `json:"nodes"`
}

type changeView struct {
	Class string `json:"class"`
	Label string `json:"label"`
}

type treeNodeView struct {
	Kind      string        `json:"kind"`
	Key       string        `json:"key"`
	Group     string        `json:"group"`
	Path      string        `json:"path"`
	Label     string        `json:"label"`
	Depth     uint32        `json:"depth"`
	Title     string        `json:"title"`
	AriaLabel string        `json:"aria_label"`
	Change    *changeView   `json:"change"`
	Stats     statsView     `json:"stats"`
	Opens     *diffPosition `json:"opens"`
}

type fileLabels struct {
	DisplayPath  string     `json:"display_path"`
	Change       changeView `json:"change"`
	FocusedLabel *string    `json:"focused_label"`
}

// changeMark is the mark of a tree row: none for a modification.
func changeMark(kind changeKind) *changeView {
	if kind == changeModified {
		return nil
	}
	class, label := kind.changeWords()
	return &changeView{Class: class, Label: label}
}

// sharedChangeKind is the change kind every file under `nodes` shares, when they share one.
func sharedChangeKind(nodes []treeNode, diff *rangeDiff) (changeKind, bool) {
	var shared changeKind
	known := false
	for _, node := range nodes {
		var kind changeKind
		if node.isDirectory {
			inner, ok := sharedChangeKind(node.children, diff)
			if !ok {
				return 0, false
			}
			kind = inner
		} else {
			kind = diff.files[node.fileIndex].changeKind
		}
		if known && shared != kind {
			return 0, false
		}
		shared, known = kind, true
	}
	return shared, known
}

// displayPath is the path a file is shown under: `old -> new` for a rename or a copy.
func displayPath(file *diffFile) string {
	if file.changeKind == changeRenamed || file.changeKind == changeCopied {
		return file.oldPath + " -> " + file.newPath
	}
	return file.path
}

// mustSignCounts is how many MUST SIGN hunks one file, or the files under one row, has and how many carry a signature.
type mustSignCounts struct {
	signed uint32
	total  uint32
}

func mustSignCountsByFile(signatures *hunkSignatures) map[string]mustSignCounts {
	counted := map[string]mustSignCounts{}
	for _, hunk := range signatures.hunks {
		entry := counted[hunk.Path]
		entry.total++
		if hunk.Signature != nil {
			entry.signed++
		}
		counted[hunk.Path] = entry
	}
	return counted
}

// countsUnder is the MUST SIGN counts of the MUST SIGN files under `nodes`, absent when none of them is MUST SIGN.
func countsUnder(nodes []treeNode, diff *rangeDiff, counts map[string]mustSignCounts) *mustSignCounts {
	var total *mustSignCounts
	for _, node := range nodes {
		var part *mustSignCounts
		if node.isDirectory {
			part = countsUnder(node.children, diff, counts)
		} else if file := &diff.files[node.fileIndex]; file.mustSign() {
			entry := counts[file.path]
			part = &entry
		}
		if part != nil {
			if total == nil {
				total = &mustSignCounts{}
			}
			total.signed += part.signed
			total.total += part.total
		}
	}
	return total
}

func plural(count uint32, singular, many string) string {
	if count == 1 {
		return singular
	}
	return many
}

// statsOf is the counts a row prints; under MUST SIGN the hunk count is signed over MUST SIGN hunks.
func statsOf(statistics treeStatistics, mustSign *mustSignCounts, showFiles, showChanges bool) statsView {
	changes := fmt.Sprintf("%d additions, %d deletions", statistics.additions, statistics.deletions)
	files := fmt.Sprintf("%d %s", statistics.files, plural(statistics.files, "file", "files"))
	view := statsView{HunksTitle: "Hunks", AriaLabel: changes}
	if mustSign != nil {
		view.HunksLabel = fmt.Sprintf("%d/%d %s", mustSign.signed, mustSign.total, plural(mustSign.total, "hunk", "hunks"))
		view.HunksTitle = "Signed / total hunks"
		view.HunksAriaLabel = pointer(fmt.Sprintf("%d signed / %d hunks", mustSign.signed, mustSign.total))
	} else {
		view.HunksLabel = fmt.Sprintf("%d %s", statistics.hunks, plural(statistics.hunks, "hunk", "hunks"))
	}
	if showFiles {
		view.FilesLabel = pointer(fmt.Sprint(statistics.files))
		view.AriaLabel = files + ", " + changes
	}
	if showChanges {
		view.AdditionsLabel = pointer(fmt.Sprintf("+%d", statistics.additions))
		view.DeletionsLabel = pointer(fmt.Sprintf("-%d", statistics.deletions))
	}
	return view
}

// firstPosition is the position opening a file shows: its first hunk, or none for a file without hunks.
func firstPosition(file *diffFile) diffPosition {
	position := diffPosition{Path: file.path}
	if len(file.hunks) > 0 {
		position.Hunk = file.hunks[0].header
	}
	return position
}

func firstPositionUnder(nodes []treeNode, diff *rangeDiff) *diffPosition {
	for _, node := range nodes {
		if node.isDirectory {
			if found := firstPositionUnder(node.children, diff); found != nil {
				return found
			}
			continue
		}
		position := firstPosition(&diff.files[node.fileIndex])
		return &position
	}
	return nil
}

func headingOf(label string, statistics treeStatistics, mustSign *mustSignCounts, first *diffPosition) groupHeadingView {
	stats := statsOf(statistics, mustSign, true, true)
	return groupHeadingView{Label: label, AriaLabel: label + " " + stats.AriaLabel, Empty: statistics.files == 0,
		FirstPosition: first, Stats: stats}
}

// subclassKey names a subclass group in a request for a directory's rows.
func subclassKey(kind fileKind, class fileClass, subclass fileSubclass) string {
	return kind.wireWord() + ":" + class.label() + ":" + subclass.label()
}

func treeViewOf(diff *rangeDiff, signatures *hunkSignatures) []kindGroupView {
	counts := mustSignCountsByFile(signatures)
	views := []kindGroupView{}
	for _, group := range diff.tree {
		var allNodes []treeNode
		for _, class := range group.classGroups {
			for _, subclass := range class.subclassGroups {
				allNodes = append(allNodes, subclass.nodes...)
			}
		}
		kindView := kindGroupView{Kind: group.kind.wireWord(),
			Heading: headingOf(group.kind.label(), group.statistics, countsUnder(allNodes, diff, counts), firstPositionUnder(allNodes, diff)),
			Classes: []classGroupView{}}
		for _, class := range group.classGroups {
			var classNodes []treeNode
			for _, subclass := range class.subclassGroups {
				classNodes = append(classNodes, subclass.nodes...)
			}
			classView := classGroupView{Key: group.kind.wireWord() + ":" + class.class.label(),
				Heading:    headingOf(class.class.label(), class.statistics, countsUnder(classNodes, diff, counts), firstPositionUnder(classNodes, diff)),
				Subclasses: []subclassGroupView{}}
			for _, subclass := range class.subclassGroups {
				key := subclassKey(group.kind, class.class, subclass.subclass)
				subclassView := subclassGroupView{Key: key,
					Heading: headingOf(subclass.subclass.label(), subclass.statistics, countsUnder(subclass.nodes, diff, counts), firstPositionUnder(subclass.nodes, diff)),
					Nodes:   []treeNodeView{}}
				for index := range subclass.nodes {
					subclassView.Nodes = append(subclassView.Nodes, nodeView(diff, key, &subclass.nodes[index], 0, counts))
				}
				classView.Subclasses = append(classView.Subclasses, subclassView)
			}
			kindView.Classes = append(kindView.Classes, classView)
		}
		views = append(views, kindView)
	}
	return views
}

func subclassGroupOf(diff *rangeDiff, group string) *subclassGroup {
	for kindIndex := range diff.tree {
		kind := &diff.tree[kindIndex]
		for classIndex := range kind.classGroups {
			class := &kind.classGroups[classIndex]
			for subclassIndex := range class.subclassGroups {
				subclass := &class.subclassGroups[subclassIndex]
				if subclassKey(kind.kind, class.class, subclass.subclass) == group {
					return subclass
				}
			}
		}
	}
	return nil
}

// childrenView is the rows of `directory` in the subclass group `group`, or false when the group holds no such directory.
func childrenView(diff *rangeDiff, signatures *hunkSignatures, group, directory string) ([]treeNodeView, bool) {
	var find func(nodes []treeNode, depth uint32) ([]treeNode, uint32, bool)
	find = func(nodes []treeNode, depth uint32) ([]treeNode, uint32, bool) {
		for _, node := range nodes {
			if !node.isDirectory {
				continue
			}
			if node.path == directory {
				return node.children, depth + 1, true
			}
			if strings.HasPrefix(directory, node.path+"/") {
				return find(node.children, depth+1)
			}
		}
		return nil, 0, false
	}
	subclass := subclassGroupOf(diff, group)
	if subclass == nil {
		return nil, false
	}
	children, depth, found := find(subclass.nodes, 0)
	if !found {
		return nil, false
	}
	counts := mustSignCountsByFile(signatures)
	views := []treeNodeView{}
	for index := range children {
		views = append(views, nodeView(diff, group, &children[index], depth, counts))
	}
	return views, true
}

// treeLocation is the subclass group that lists `file` and the directories, outermost first, that hold it there.
func treeLocation(diff *rangeDiff, file *diffFile) (string, []string) {
	group := subclassKey(file.classification.kind, file.classification.class, file.classification.subclass)
	directories := []string{}
	nodes := subclassGroupOf(diff, group).nodes
	for {
		var next *treeNode
		for index := range nodes {
			if nodes[index].isDirectory && strings.HasPrefix(file.path, nodes[index].path+"/") {
				next = &nodes[index]
				break
			}
		}
		if next == nil {
			return group, directories
		}
		directories = append(directories, next.path)
		nodes = next.children
	}
}

func nodeView(diff *rangeDiff, group string, node *treeNode, depth uint32, counts map[string]mustSignCounts) treeNodeView {
	if node.isDirectory {
		stats := statsOf(node.statistics, countsUnder(node.children, diff, counts), true, false)
		// A directory is marked only when all its files are added or all are deleted.
		var change *changeView
		if kind, shared := sharedChangeKind(node.children, diff); shared && (kind == changeAdded || kind == changeDeleted) {
			change = changeMark(kind)
		}
		view := treeNodeView{Kind: "directory", Key: "directory:" + group + ":" + node.path, Group: group, Path: node.path,
			Label: node.label, Depth: depth, Title: node.path, AriaLabel: stats.AriaLabel, Change: change, Stats: stats}
		if change != nil {
			view.Title = change.Label + ": " + node.path
			view.AriaLabel = change.Label + " " + stats.AriaLabel
		}
		return view
	}
	file := &diff.files[node.fileIndex]
	var mustSign *mustSignCounts
	if file.mustSign() {
		entry := counts[file.path]
		mustSign = &entry
	}
	stats := statsOf(fileStatistics(file), mustSign, false, true)
	change := changeMark(file.changeKind)
	opens := firstPosition(file)
	view := treeNodeView{Kind: "file", Key: "file:" + group + ":" + node.path, Group: group, Path: node.path,
		Label: node.label, Depth: depth, Title: displayPath(file), AriaLabel: displayPath(file) + " " + stats.AriaLabel,
		Change: change, Stats: stats, Opens: &opens}
	if change != nil {
		view.Title = change.Label + ": " + displayPath(file)
		view.AriaLabel = change.Label + " " + displayPath(file) + " " + stats.AriaLabel
	}
	return view
}

func labelsOf(file *diffFile) fileLabels {
	class, label := file.changeKind.changeWords()
	labels := fileLabels{DisplayPath: displayPath(file), Change: changeView{Class: class, Label: label}}
	if !file.isBinary {
		labels.FocusedLabel = pointer("Full file · focused")
	}
	return labels
}
