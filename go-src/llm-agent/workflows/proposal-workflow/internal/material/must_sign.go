package material

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"proposal-workflow/internal/workflow"
)

func (b Builder) MustSign(ctx context.Context, t workflow.Task, head string) ([]string, map[string]string, error) {
	patternPath := b.Config.MustSignPatterns
	if patternPath == "" {
		patternPath = "docs/must-sign-patterns.txt"
	}
	if !filepath.IsAbs(patternPath) {
		patternPath = filepath.Join(b.Config.Repository, b.Config.Project, patternPath)
	}
	patterns, err := os.ReadFile(patternPath)
	if err != nil {
		return nil, nil, err
	}
	matchers := []*regexp.Regexp{}
	for _, line := range strings.Split(string(patterns), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var expression strings.Builder
		expression.WriteString("^")
		for i := 0; i < len(line); i++ {
			switch line[i] {
			case '*':
				if i+1 < len(line) && line[i+1] == '*' {
					expression.WriteString(".*")
					i++
				} else {
					expression.WriteString("[^/]*")
				}
			case '?':
				expression.WriteString("[^/]")
			default:
				expression.WriteString(regexp.QuoteMeta(string(line[i])))
			}
		}
		expression.WriteString("$")
		matcher, err := regexp.Compile(expression.String())
		if err != nil {
			return nil, nil, err
		}
		matchers = append(matchers, matcher)
	}
	changes, err := b.Repository.Changes(ctx, t.Directory, t.Work.Base, head)
	if err != nil {
		return nil, nil, err
	}
	changed := map[string]bool{}
	required := map[string]bool{}
	reasons := map[string]string{}
	paths := []string{}
	for _, path := range changes {
		changed[path] = true
		for _, matcher := range matchers {
			if matcher.MatchString(path) {
				required[path] = true
				paths = append(paths, path)
				break
			}
		}
	}
	additionPath := filepath.ToSlash(filepath.Join(t.Work.Folder, "must-sign-additions.json"))
	listing, err := b.Repository.Git(ctx, t.Directory, "ls-tree", "--name-only", head, "--", additionPath)
	if err != nil {
		return nil, nil, err
	}
	if listing != "" {
		raw, err := b.Repository.Read(ctx, t.Directory, head, additionPath)
		if err != nil {
			return nil, nil, err
		}
		var additions Additions
		if err = Decode(raw, &additions); err != nil {
			return nil, nil, err
		}
		for _, a := range additions.Additions {
			if a.Path == "Cargo.lock" || !changed[a.Path] || required[a.Path] || strings.TrimSpace(a.Reason) == "" || strings.Contains(a.Reason, "\n") {
				return nil, nil, fmt.Errorf("invalid MUST SIGN addition %q", a.Path)
			}
			required[a.Path] = true
			paths = append(paths, a.Path)
			reasons[a.Path] = a.Reason
		}
	}
	if t.Work.Parent != "" && t.Work.Parent != t.Work.Base {
		prior, err := b.Repository.Git(ctx, t.Directory, "ls-tree", "--name-only", t.Work.Parent, "--", additionPath)
		if err != nil {
			return nil, nil, err
		}
		if prior != "" {
			raw, err := b.Repository.Read(ctx, t.Directory, t.Work.Parent, additionPath)
			if err != nil {
				return nil, nil, err
			}
			var previous Additions
			if err = Decode(raw, &previous); err != nil {
				return nil, nil, err
			}
			for _, a := range previous.Additions {
				if changed[a.Path] && !required[a.Path] {
					return nil, nil, fmt.Errorf("cannot remove prior MUST SIGN addition while file remains changed: %s", a.Path)
				}
			}
		}
	}
	return paths, reasons, nil
}
