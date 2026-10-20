package reviewer

import (
	"fmt"
	"path"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"sort"
	"strings"
)

func position(h api.Hunk) object { return object{"path": h.Path, "hunk": header(h)} }
func (c *Candidate) selectedFiles(group, prefix string) []*File {
	out := []*File{}
	for _, p := range c.Paths {
		f := c.Files[p]
		if (group == "" || f.Group == group || strings.HasPrefix(f.Group, group+":")) && (prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/")) {
			out = append(out, f)
		}
	}
	return out
}
func stats(files []*File, r service.ReviewRevision, compact bool) object {
	added, deleted, hunks, signed := 0, 0, 0, 0
	required := false
	for _, f := range files {
		added += f.Added
		deleted += f.Deleted
		hunks += len(f.Hunks)
		required = required || f.Required
		for _, h := range f.Hunks {
			if _, ok := r.Signatures[h.ID]; ok {
				signed++
			}
		}
	}
	word := "hunks"
	if hunks == 1 {
		word = "hunk"
	}
	count := fmt.Sprintf("%d %s", hunks, word)
	title := "Hunks"
	var aria any
	if required {
		count = fmt.Sprintf("%d/%d %s", signed, hunks, word)
		title = "Signed / total hunks"
		aria = fmt.Sprintf("%d signed / %d hunks", signed, hunks)
	}
	var additions, deletions any = fmt.Sprintf("+%d", added), fmt.Sprintf("-%d", deleted)
	if compact {
		additions = nil
		deletions = nil
	}
	return object{"files_label": fmt.Sprint(len(files)), "hunks_label": count, "hunks_title": title, "hunks_aria_label": aria, "additions_label": additions, "deletions_label": deletions, "aria_label": fmt.Sprintf("%d files, %d additions, %d deletions", len(files), added, deleted)}
}
func heading(label string, files []*File, r service.ReviewRevision) object {
	st := stats(files, r, false)
	var first any
	if len(files) > 0 && len(files[0].Hunks) > 0 {
		first = position(files[0].Hunks[0])
	}
	return object{"label": label, "aria_label": label + " " + st["aria_label"].(string), "empty": len(files) == 0, "first_position": first, "stats": st}
}
func (c *Candidate) nodes(group, directory string, r service.ReviewRevision) []object {
	files := c.selectedFiles(group, directory)
	prefix := directory
	if prefix != "" {
		prefix += "/"
	}
	branches := map[string][]*File{}
	for _, f := range files {
		rest := strings.TrimPrefix(f.Path, prefix)
		name := strings.Split(rest, "/")[0]
		branches[prefix+name] = append(branches[prefix+name], f)
	}
	keys := []string{}
	for k := range branches {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := branches[keys[i]], branches[keys[j]]
		ad, bd := len(a) > 1 || a[0].Path != keys[i], len(b) > 1 || b[0].Path != keys[j]
		if ad != bd {
			return ad
		}
		return keys[i] < keys[j]
	})
	nodes := []object{}
	for _, nodePath := range keys {
		selected := branches[nodePath]
		directoryNode := len(selected) > 1 || selected[0].Path != nodePath
		// Collapse a chain with no branching, matching the shared explorer.
		if directoryNode {
			for {
				next := ""
				same := true
				for _, f := range selected {
					rest := strings.TrimPrefix(f.Path, nodePath+"/")
					if !strings.Contains(rest, "/") {
						same = false
						break
					}
					part := strings.Split(rest, "/")[0]
					if next != "" && next != part {
						same = false
						break
					}
					next = part
				}
				if !same || next == "" {
					break
				}
				nodePath += "/" + next
			}
		}
		kind := "file"
		var opens any
		var change any = selected[0].Change
		if selected[0].Change["class"] == "modify" {
			change = nil
		}
		title := nodePath
		if directoryNode {
			kind = "directory"
			change = nil
			allAdded := true
			for _, f := range selected {
				allAdded = allAdded && f.Change["class"] == "add"
			}
			if allAdded {
				change = object{"class": "add", "label": "Added"}
				title = "Added: " + nodePath
			}
		} else if len(selected[0].Hunks) > 0 {
			opens = position(selected[0].Hunks[0])
		}
		st := stats(selected, r, directoryNode)
		if !directoryNode {
			st["files_label"] = nil
		}
		depth := 0
		if directory != "" {
			for i, dir := range selected[0].Directories {
				if dir == directory {
					depth = i + 1
					break
				}
			}
		}
		nodes = append(nodes, object{"kind": kind, "key": kind + ":" + group + ":" + nodePath, "group": group, "path": nodePath, "label": strings.TrimPrefix(nodePath, prefix), "depth": depth, "title": title, "aria_label": nodeAria(change, st), "change": change, "stats": st, "opens": opens})
	}
	return nodes
}
func (c *Candidate) diff(r service.ReviewRevision, snap service.ReviewSnapshot) object {
	tree := []object{}
	for _, kind := range []string{"must-sign", "test-data", "config", "docs", "src"} {
		files := c.selectedFiles(kind, "")
		if len(files) == 0 {
			continue
		}
		classes := []object{}
		for _, class := range classOrder {
			group := kind + ":" + class + ":Other"
			members := c.selectedFiles(group, "")
			if len(members) == 0 {
				continue
			}
			classes = append(classes, object{"key": kind + ":" + class, "heading": heading(class, members, r), "subclasses": []object{{"key": group, "heading": heading("Other", members, r), "nodes": c.nodes(group, "", r)}}})
		}
		label := map[string]string{"must-sign": "MUST SIGN", "test-data": "Test data", "config": "Config", "docs": "Docs", "src": "Source"}[kind]
		groupHeading := heading(label, files, r)
		if len(classes) > 0 {
			groupHeading["first_position"] = classes[0]["heading"].(object)["first_position"]
		}
		tree = append(tree, object{"kind": kind, "heading": groupHeading, "classes": classes})
	}
	var first any
	if len(tree) > 0 {
		first = tree[0]["heading"].(object)["first_position"]
	}
	rulings := []object{}
	for _, d := range c.Material.Doubts {
		rulings = append(rulings, object{"identifier": d.ID, "ruling": nullable(r.Rulings[d.ID])})
	}
	comments := len(c.History)
	for _, e := range snap.Entries {
		if e.Proposal == c.Material.Proposal && e.Kind != "sign" && e.Kind != "sign_under" {
			comments++
		}
	}
	return object{"candidate": c.Subject, "generation": snap.Sequence, "tree": tree, "first_position": first, "sign_presses": len(r.Signatures), "write": nil, "opened": r.Opened, "finding_rulings": rulings, "proposal_wide_findings": c.wideFindings("", r), "review_comment_count": comments}
}

// next walks the displayed hunk order after the current hunk, wrapping once.
// Signing always leaves that hunk, even while one of its Doubts remains unruled.
func (c *Candidate) next(r service.ReviewRevision, current string) any {
	start := 0
	if index, ok := c.HunkIndex[current]; ok {
		start = index + 1
	}
	for offset := 0; offset < len(c.Order); offset++ {
		h := c.Order[(start+offset)%len(c.Order)]
		if h.ID == current {
			continue
		}
		if c.needsAction(h, r) {
			return position(h)
		}
	}
	return nil
}
func (c *Candidate) needsAction(h api.Hunk, r service.ReviewRevision) bool {
	if h.MustSign {
		if _, ok := r.Signatures[h.ID]; !ok {
			return true
		}
	}
	for _, finding := range c.hunkFindings(h, r) {
		if finding["ruling"] == nil {
			return true
		}
	}
	return false
}
func (c *Candidate) fileView(f *File, r service.ReviewRevision, snap service.ReviewSnapshot, cited *Citation) object {
	hunks := []object{}
	ordinal := 0
	ordinals := map[string]int{}
	for _, h := range c.Material.Hunks {
		if h.MustSign {
			ordinal++
			ordinals[h.ID] = ordinal
		}
	}
	for i, h := range f.Hunks {
		var signature any
		var ordinal any
		if h.MustSign {
			ordinal = ordinals[h.ID]
			state := object{"path": h.Path, "hunk": header(h), "range": header(h), "ordinal": ordinal, "changedLines": changed(h)}
			if sig, ok := r.Signatures[h.ID]; ok {
				v := object{"signer": sig.Signer, "at": sig.At}
				if sig.CarriedFrom != "" {
					v["carried"] = true
					v["signedAtCandidate"] = sig.SignedAtCandidate
				}
				state["signature"] = v
			}
			signature = state
		}
		var up, down any
		walk := c.Order
		for index, other := range walk {
			if other.ID == h.ID {
				if index > 0 {
					up = position(walk[index-1])
				}
				if index+1 < len(walk) {
					down = position(walk[index+1])
				}
				break
			}
		}
		findings := c.hunkFindings(h, r)
		var empty any
		if len(findings) == 0 {
			empty = "No doubts."
		}
		sections := []object{{"label": "Doubts", "heading": fmt.Sprintf("Doubts · %d", len(findings)), "empty_notice": empty, "findings": findings}}
		var afterRuling any
		if !c.needsAction(h, r) {
			afterRuling = c.next(r, h.ID)
		}
		label := fmt.Sprintf("#%d", i+1)
		if ordinal != nil {
			label = fmt.Sprintf("#%v", ordinal)
		}
		hunks = append(hunks, object{"header": header(h), "header_line": h.Header, "label": label, "must_sign_hunk_ordinal": ordinal, "signature": signature, "review_sections": sections, "file_wide_findings": c.wideFindings(f.Path, r), "proposal_wide_findings": c.wideFindings("", r), "up": up, "down": down, "next_action": c.next(r, h.ID), "after_sign": c.next(r, h.ID), "after_ruling": afterRuling})
	}
	directories := []string{}
	for dir := path.Dir(f.Path); dir != "."; dir = path.Dir(dir) {
		directories = append(directories, dir)
	}
	operations := []object{}
	for i := len(directories) - 1; i >= 0; i-- {
		dir := directories[i]
		count := 0
		for _, h := range c.Material.Hunks {
			if h.MustSign && strings.HasPrefix(h.Path, dir+"/") {
				if _, ok := r.Signatures[h.ID]; !ok {
					count++
				}
			}
		}
		operations = append(operations, object{"directory": dir, "label": "under " + dir, "unsigned_count": count})
	}
	var reason any
	for _, h := range f.Hunks {
		if h.Reason != "" {
			reason = object{"label": "MUST SIGN · added by the Worker", "reason": h.Reason}
			break
		}
	}
	var citation any
	if cited != nil {
		lines := []int{}
		end := cited.Line
		if cited.End != nil {
			end = *cited.End
		}
		for n := cited.Line; n <= end; n++ {
			lines = append(lines, n)
		}
		var focus any
		for _, b := range f.Blocks {
			for _, l := range b.Lines {
				if l.New != nil && *l.New == cited.Line {
					focus = b.Hunk
				}
			}
		}
		citation = object{"line": cited.Line, "hunk": focus, "lines": lines}
	}
	return object{"generation": snap.Sequence, "path": f.Path, "labels": object{"display_path": f.Path, "change": f.Change, "focused_label": "Full file · focused"}, "must_sign_reason": reason, "tree_group": f.Group, "tree_directories": f.Directories, "hunks": hunks, "cited": citation, "file_wide_findings": c.wideFindings(f.Path, r), "next_action": c.next(r, ""), "batch_sign_menu": object{"operations": operations}}
}
func changed(h api.Hunk) int {
	count := 0
	for _, line := range strings.Split(h.Patch, "\n") {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			count++
		}
	}
	return count
}

type Citation struct {
	Line int  `json:"line"`
	End  *int `json:"end_line"`
}

func (c *Candidate) walk() []api.Hunk {
	out := []api.Hunk{}
	for _, kind := range []string{"must-sign", "test-data", "config", "docs", "src"} {
		for _, class := range classOrder {
			for _, f := range c.selectedFiles(kind+":"+class+":Other", "") {
				out = append(out, f.Hunks...)
			}
		}
	}
	return out
}

func nodeAria(change any, stats object) string {
	label := stats["aria_label"].(string)
	if change != nil {
		label = change.(object)["label"].(string) + " " + label
	}
	return label
}

// Explorer order visits directories before their sibling files.
func filePathLess(a, b string) bool {
	left, right := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] == right[i] {
			continue
		}
		ld, rd := i < len(left)-1, i < len(right)-1
		if ld != rd {
			return ld
		}
		return left[i] < right[i]
	}
	return a < b
}
