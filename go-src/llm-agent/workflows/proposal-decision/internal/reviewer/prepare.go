package reviewer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"proposal-decision/internal/service"
	"proposal-platform/api"
	"proposal-platform/notebook"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func prepare(ctx context.Context, m api.Candidate, repository string) (*Candidate, error) {
	c := &Candidate{Material: m, Subject: Subject{number(m.Proposal), m.ID}, Documents: map[string]api.Document{}, Sources: map[string]string{}, Files: map[string]*File{}, Label: "R0", Ordinal: "1", History: []object{}}
	reads := map[string]bool{}
	for _, h := range m.Hunks {
		if c.Files[h.Path] == nil {
			// The shared Diff UI requires an array even for root-level files.
			c.Files[h.Path] = &File{Path: h.Path, Required: h.MustSign, Directories: []string{}}
			c.Paths = append(c.Paths, h.Path)
		}
		c.Files[h.Path].Hunks = append(c.Files[h.Path].Hunks, h)
		reads[m.Base+":"+h.Path] = true
		reads[m.ID+":"+h.Path] = true
		if strings.Contains(h.Path, "/proposals/") && strings.Contains(h.Path, "/"+m.Proposal+"/") {
			c.Folder = strings.Split(h.Path, "/"+m.Proposal+"/")[0] + "/" + m.Proposal
		}
	}
	for _, d := range m.Documents {
		c.Documents[d.ID] = d
		if d.Kind == "tests" || d.Kind == "related" || d.Kind == "criteria" {
			parsed, err := notebook.Parse(d.ID, d.Text)
			if err != nil {
				return nil, err
			}
			for _, group := range parsed.Groups {
				for _, item := range group.Items {
					oid := m.ID
					if item.Source.Revision != "" {
						oid = item.Source.Revision
					}
					reads[oid+":"+item.Source.Path] = true
				}
			}
		}
		if d.Kind == "provenance" {
			if match := regexp.MustCompile(`(?i)batch (\d+) #(\d+)`).FindStringSubmatch(d.Text); match != nil {
				c.Batch = match[1]
				c.Ordinal = match[2]
			}
			if match := regexp.MustCompile(`(?i)revision (\d+)`).FindStringSubmatch(d.Text); match != nil {
				n, _ := strconv.Atoi(match[1])
				c.Label = fmt.Sprintf("R%d", max(0, n-1))
			}
		}
	}
	if m.Folder != "" {
		c.Folder = m.Folder
	}
	if c.Folder != "" {
		parts := strings.Split(c.Folder, "/")
		if len(parts) >= 4 {
			c.Date = strings.Join(parts[len(parts)-4:len(parts)-1], "-")
		}
		c.Branch = "temp_" + strings.NewReplacer("/", "_", "-", "_").Replace(strings.Split(c.Folder, "/proposals/")[0][len("projects/"):]) + "_proposal_" + m.Proposal
	}
	if m.Source != nil {
		// The served repository shares the delivered Git object store. Never rely on a
		// worker checkout still existing or on its current worktree contents.
		sources, err := readBlobs(ctx, repository, reads)
		if err != nil {
			return nil, err
		}
		c.Sources = sources
	}
	sort.Slice(c.Paths, func(i, j int) bool { return filePathLess(c.Paths[i], c.Paths[j]) })
	for _, file := range c.Files {
		if err := file.prepare(ctx, c); err != nil {
			return nil, err
		}
	}
	for _, file := range c.Files {
		directory := ""
		for {
			next := ""
			for _, node := range c.nodes(file.Group, directory, service.ReviewRevision{}) {
				p := node["path"].(string)
				if node["kind"] == "directory" && strings.HasPrefix(file.Path, p+"/") {
					next = p
					break
				}
			}
			if next == "" {
				break
			}
			file.Directories = append(file.Directories, next)
			directory = next
		}
	}
	c.Order = c.walk()
	c.HunkIndex = map[string]int{}
	for i, h := range c.Order {
		c.HunkIndex[h.ID] = i
	}
	parsedNotebook, err := c.notebook(c.Documents["tests"])
	c.Notebook = parsedNotebook
	if err != nil {
		return nil, err
	}
	c.Outcome = c.outcome()
	c.History = c.history()
	c.ProjectDiff = c.projectDiff()
	if err := c.prepareProjectPage(ctx, repository); err != nil {
		return nil, err
	}
	return c, nil
}

// readBlobs uses one bounded Git process for all pinned inputs. Missing paths are
// expected for added/deleted files; malformed or oversized output fails launch.
func readBlobs(ctx context.Context, repository string, requested map[string]bool) (map[string]string, error) {
	keys := make([]string, 0, len(requested))
	for k := range requested {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	command := readGit(ctx, repository, "cat-file", "--batch")
	command.Stdin = strings.NewReader(strings.Join(keys, "\n") + "\n")
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err = command.Start(); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(pipe, 128<<20))
	wait := command.Wait()
	if err != nil {
		return nil, err
	}
	if wait != nil {
		return nil, fmt.Errorf("read pinned sources: %s", stderr.String())
	}
	if len(data) >= 128<<20 {
		return nil, fmt.Errorf("pinned sources exceed 128 MiB")
	}
	reader := bufio.NewReader(bytes.NewReader(data))
	result := map[string]string{}
	for _, key := range keys {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			continue
		}
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("unexpected Git blob header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size > 24<<20 {
			return nil, fmt.Errorf("invalid blob size")
		}
		body := make([]byte, size)
		if _, err = io.ReadFull(reader, body); err != nil {
			return nil, err
		}
		if end, err := reader.ReadByte(); err != nil || end != '\n' {
			return nil, fmt.Errorf("invalid blob framing")
		}
		result[key] = string(body)
	}
	return result, nil
}
func (c *Candidate) notebook(doc api.Document) (object, error) {
	parsed, err := notebook.Parse(path.Join(c.Folder, "tests.md"), doc.Text)
	if err != nil {
		return nil, err
	}
	err = parsed.Bind(c.Material.ID, func(oid, path string) (string, error) {
		source, ok := c.Sources[oid+":"+path]
		if !ok {
			return "", fmt.Errorf("missing pinned notebook source %s", path)
		}
		return source, nil
	})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	var result object
	err = json.Unmarshal(raw, &result)
	return result, err
}
func (c *Candidate) surface(id, title string) object {
	noticeText := "No " + title + " document."
	if c.stoppedAt(id) {
		noticeText = "This candidate stops before the " + title + " surface."
	}
	content := object{"kind": "absent", "notice": noticeText}
	if notice, ok := c.Documents["notice"]; ok && notice.Title == id {
		return object{"identifier": id, "heading": title, "stop": stop(c.Material.Kind), "content": object{"kind": "question", "path": path.Join(c.Folder, "question.md"), "markdown": notice.Text, "links": questionLinks(notice.Text)}}
	}
	if doc, ok := c.Documents[id]; ok {
		if doc.HTML != "" {
			content = object{"kind": "kathy", "document": object{"path": path.Join(c.Folder, id+".kathy.json"), "document": object{"rendered_html": doc.HTML}}}
		}
		if doc.HTML == "" {
			var wire any
			if json.Unmarshal([]byte(doc.Text), &wire) == nil {
				content = object{"kind": "kathy", "document": object{"path": id, "document": wire}}
			} else {
				content = object{"kind": "question", "path": id, "markdown": doc.Text, "links": []any{}}
			}
		}
	}
	return object{"identifier": id, "heading": title, "stop": nil, "content": content}
}
func (c *Candidate) main() object {
	return object{"candidate": c.Subject, "columns": [][]object{{c.surface("problem", "Problem"), c.surface("scope", "Scope")}, {c.surface("diagnosis", "Diagnosis"), c.surface("design", "Design")}, {c.surface("implementation-plan", "Implementation Plan")}}, "bypass": nil}
}

func (c *Candidate) stoppedAt(id string) bool {
	if c.Material.Kind == "complete" {
		return false
	}
	notice, ok := c.Documents["notice"]
	if !ok {
		return false
	}
	order := []string{"problem", "scope", "diagnosis", "related", "criteria", "design", "implementation-plan", "tests", "effect"}
	stopped := false
	for _, surface := range order {
		if surface == notice.Title {
			stopped = true
		}
		if surface == id {
			return stopped
		}
	}
	return false
}

var questionLink = regexp.MustCompile(`\[[^]]+\]\(([^)#]+)(?:#L([0-9]+)(?:-L([0-9]+))?)?\)`)

func questionLinks(text string) []object {
	links := []object{}
	for _, match := range questionLink.FindAllStringSubmatch(text, -1) {
		file := match[1]
		if strings.Contains(file, ":") || strings.HasPrefix(file, "/") || strings.Contains(file, "..") {
			continue
		}
		start, end := 1, 1
		if match[2] != "" {
			start, _ = strconv.Atoi(match[2])
			end = start
		}
		if match[3] != "" {
			end, _ = strconv.Atoi(match[3])
		}
		href := file
		if match[2] != "" {
			href += "#L" + match[2]
		}
		if match[3] != "" {
			href += "-L" + match[3]
		}
		links = append(links, object{"href": href, "path": file, "start_line": start, "end_line": end})
	}
	return links
}
