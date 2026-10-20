package material

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Manifest describes the committed format-4 proposal surfaces.
type Manifest struct {
	Format       int               `json:"format"`
	Base         string            `json:"base"`
	Kind         string            `json:"kind"`
	Surfaces     map[string]string `json:"surfaces"`
	Question     *Notice           `json:"question,omitempty"`
	ChowProblems *Notice           `json:"chow_problems,omitempty"`
}

func (m Manifest) Validate() error {
	names := []string{"Problem", "Scope", "Diagnosis", "Related", "Criteria", "Design", "Implementation Plan", "Diff", "Tests", "Effect"}
	allowed := map[string]bool{}
	missing := ""
	for _, name := range names {
		allowed[name] = true
		status, ok := m.Surfaces[name]
		if !ok {
			if missing == "" {
				missing = name
			}
			continue
		}
		if missing != "" {
			return fmt.Errorf("surface %s follows unfinished %s", name, missing)
		}
		if status != "updated" && status != "revalidated" {
			return fmt.Errorf("invalid status for %s", name)
		}
	}
	for name := range m.Surfaces {
		if !allowed[name] {
			return fmt.Errorf("unknown surface %s", name)
		}
	}
	switch m.Kind {
	case "complete":
		if missing != "" || m.Question != nil || m.ChowProblems != nil {
			return fmt.Errorf("complete manifest must contain exactly ten surfaces and no notice")
		}
	case "incomplete", "chow_problems":
		notice := m.Question
		other := m.ChowProblems
		if m.Kind == "chow_problems" {
			notice, other = other, notice
		}
		if notice == nil || other != nil || missing == "" || strings.ToLower(strings.ReplaceAll(missing, " ", "-")) != notice.Tab || !filepath.IsLocal(notice.Document) {
			return fmt.Errorf("notice must name the first unfinished surface and a local document")
		}
	default:
		return fmt.Errorf("invalid manifest kind %s", m.Kind)
	}
	return nil
}
