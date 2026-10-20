package material

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"proposal-platform/api"
	"proposal-workflow/internal/command"
	"proposal-workflow/internal/project"
	"proposal-workflow/internal/repository"
	"proposal-workflow/internal/workflow"
)

// Builder materializes pinned surfaces and review documents once per delivery.
type Builder struct {
	Config     *project.Config
	Repository repository.Repository
	Commands   command.Runner
	Chow       string
	Renderer   string
}

func Decode(text string, value any) error {
	d := json.NewDecoder(strings.NewReader(text))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func (b Builder) Render(ctx context.Context, t workflow.Task, oid, document string) (string, error) {
	input, _ := json.Marshal(map[string]string{"repository": t.Directory, "oid": oid, "document": document})
	return b.Commands.Run(ctx, b.Config.Repository, nil, string(input), "", "bun", b.Renderer)
}
func (b Builder) Doubts(ctx context.Context, t workflow.Task, findings []api.Doubt) ([]api.Doubt, error) {
	for i := range findings {
		d := &findings[i]
		var prose []string
		currentPath := ""
		for _, line := range strings.Split(d.Text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "## ") {
				currentPath = strings.TrimSpace(strings.TrimPrefix(line, "## "))
				if d.Path == "" {
					d.Path = currentPath
				}
				d.Anchors = append(d.Anchors, api.Anchor{Path: currentPath})
				continue
			}
			if strings.HasPrefix(line, "### @@ ") {
				header := strings.TrimPrefix(line, "### ")
				if currentPath == "" {
					return nil, fmt.Errorf("hunk anchor has no file")
				}
				if d.Header == "" {
					d.Header = header
				}
				if len(d.Anchors) > 0 && d.Anchors[len(d.Anchors)-1].Header == "" {
					d.Anchors[len(d.Anchors)-1].Header = header
				} else {
					d.Anchors = append(d.Anchors, api.Anchor{Path: currentPath, Header: header})
				}
				continue
			}
			if before, question, found := strings.Cut(line, "决定:"); found {
				d.Question = strings.TrimSpace(question)
				line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(before), "—"))
				if line == "" {
					continue
				}
			}
			prose = append(prose, line)
		}
		raw, err := b.Commands.Run(ctx, t.Directory, nil, strings.Join(prose, "\n"), "", b.Chow, "-")
		if err != nil {
			return nil, err
		}
		html, err := b.Render(ctx, t, t.Candidate.ID, raw)
		if err != nil {
			return nil, err
		}
		d.Document = &api.Document{ID: d.ID, Title: d.ID, Kind: "doubt", Text: d.Text, HTML: html}
	}
	return findings, nil
}
func (b Builder) Build(ctx context.Context, t workflow.Task, head string) (*api.Candidate, error) {
	raw, err := b.Repository.Read(ctx, t.Directory, head, t.Work.Folder+"/revision.json")
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err = Decode(raw, &manifest); err != nil {
		return nil, err
	}
	if manifest.Format != 4 || manifest.Base != t.Work.Parent || manifest.Kind != t.Candidate.Kind {
		return nil, fmt.Errorf("revision.json must declare format 4, base %s, kind %s", t.Work.Parent, t.Candidate.Kind)
	}
	if err = manifest.Validate(); err != nil {
		return nil, err
	}
	material := *t.Candidate
	material.ID = head
	material.Folder = t.Work.Folder
	material.Base = t.Work.Base
	material.Documents = nil
	for _, doc := range t.Candidate.Documents {
		if doc.Kind == "outcome" || doc.Kind == "outcome-index" || doc.Kind == "outcome-records" || doc.Kind == "role" || doc.Kind == "rule-conflict" {
			material.Documents = append(material.Documents, doc)
		}
	}
	if manifest.Kind == "complete" && t.Work.Outcome == "" {
		return nil, fmt.Errorf("complete candidate has no measured Outcome")
	}
	material.Reviewed = t.Work.Reviewed
	required, reasons, err := b.MustSign(ctx, t, head)
	if err != nil {
		return nil, err
	}
	material.Source = &api.Source{Repository: t.Directory, Base: t.Work.Base, Commit: head, MustSign: required, MustSignReasons: reasons}
	names := []string{"Problem", "Scope", "Diagnosis", "Related", "Criteria", "Design", "Implementation Plan", "Diff", "Tests", "Effect"}
	for _, name := range names {
		status, exists := manifest.Surfaces[name]
		if !exists {
			if manifest.Kind == "complete" {
				return nil, fmt.Errorf("missing surface %s", name)
			}
			continue
		}
		if status != "updated" && status != "revalidated" {
			return nil, fmt.Errorf("invalid surface status %s", status)
		}
		if name == "Diff" {
			continue
		}
		id := strings.ReplaceAll(strings.ToLower(name), " ", "-")
		if b.Config.GoSurfaces == "pdf" {
			// The independent project pages consume committed PDF and text assets.
			// Validate those exact assets here; the decision server pins and caches
			// them from Source at delivery, without an intermediate sidecar or render.
			if err := b.projectPageSurface(ctx, t, head, id); err != nil {
				return nil, err
			}
			continue
		}
		extension := ".kathy.json"
		if name == "Related" || name == "Criteria" || name == "Tests" {
			extension = ".md"
		}
		text, err := b.Repository.Read(ctx, t.Directory, head, t.Work.Folder+"/"+id+extension)
		if err != nil {
			return nil, err
		}
		doc := api.Document{ID: id, Kind: id, Title: name, Text: text}
		if extension == ".kathy.json" {
			doc.HTML, err = b.Render(ctx, t, head, text)
			if err != nil {
				return nil, err
			}
		} else {
			doc.HTML, err = b.Notebook(ctx, t, head, text)
			if err != nil {
				return nil, err
			}
		}
		material.Documents = append(material.Documents, doc)
	}
	for _, notice := range []*Notice{manifest.Question, manifest.ChowProblems} {
		if notice != nil {
			text, err := b.Repository.Read(ctx, t.Directory, head, t.Work.Folder+"/"+notice.Document)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf("empty decision notice")
			}
			material.Documents = append(material.Documents, api.Document{ID: "notice", Title: notice.Tab, Kind: "notice", Text: text})
		}
	}
	return &material, nil
}

func (b Builder) projectPageSurface(ctx context.Context, t workflow.Task, head, id string) error {
	extension := ".md"
	switch id {
	case "problem", "diagnosis", "design", "implementation-plan", "effect":
		extension = ".pdf"
	}
	name := t.Work.Folder + "/" + id + extension
	content, err := b.Repository.Read(ctx, t.Directory, head, name)
	if err != nil {
		return err
	}
	if extension == ".pdf" && !strings.HasPrefix(content, "%PDF-") {
		return fmt.Errorf("%s is not a PDF", name)
	}
	return nil
}
