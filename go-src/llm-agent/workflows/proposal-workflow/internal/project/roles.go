package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Roles owns the assembled role instructions, independent of a slot's AGENTS.md.
type Roles struct{ Documents map[string]string }

func ReadRoles(c *Config) (*Roles, error) {
	result := &Roles{Documents: map[string]string{}}
	for _, name := range []string{"worker", "reviewer", "nitpicker", "commissar"} {
		document, err := AssembleRole(c.Repository, c.Project, name)
		if err != nil {
			return nil, err
		}
		result.Documents[name] = document
	}
	return result, nil
}

// AssembleRole resolves source links before combining common and project charters.
func AssembleRole(repository, project, role string) (string, error) {
	if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(role) || role == "fragments" {
		return "", fmt.Errorf("invalid role %q", role)
	}
	repository, err := filepath.EvalSymlinks(repository)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(project) {
		project = filepath.Join(repository, project)
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(repository, project)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("project is outside repository")
	}
	common := filepath.Join(repository, "projects/common/roles")
	if info, err := os.Stat(filepath.Join(common, role)); err != nil || !info.IsDir() {
		return "", fmt.Errorf("no such role %s", role)
	}
	files := []string{filepath.Join(common, "AGENTS_PREFIX.md")}
	optional := func(path string) error {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("not a file: %s", path)
		}
		files = append(files, path)
		return nil
	}
	for _, path := range []string{filepath.Join(common, role, "AGENTS_PREFIX.md"), filepath.Join(project, "roles/AGENTS_PREFIX.md")} {
		if err := optional(path); err != nil {
			return "", err
		}
	}
	for _, scope := range []string{"", role} {
		list := filepath.Join(project, "roles", scope, "fragments.txt")
		data, err := os.ReadFile(list)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, name := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			if name == "" || strings.HasPrefix(name, "#") {
				continue
			}
			if !regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`).MatchString(name) {
				return "", fmt.Errorf("%s: invalid fragment %q", list, name)
			}
			files = append(files, filepath.Join(common, scope, "fragments", name+".md"))
		}
	}
	if err := optional(filepath.Join(project, "roles", role, "AGENTS_FRAGMENT.md")); err != nil {
		return "", err
	}
	link := regexp.MustCompile(`(\[[^\]]*\])\(([^)]+)\)`)
	parts := make([]string, 0, len(files))
	for _, source := range files {
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		parts = append(parts, link.ReplaceAllStringFunc(string(data), func(match string) string {
			fields := link.FindStringSubmatch(match)
			target := fields[2]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") || filepath.IsAbs(target) {
				return match
			}
			path, anchor, hasAnchor := strings.Cut(target, "#")
			rel, err := filepath.Rel(repository, filepath.Join(filepath.Dir(source), path))
			if err != nil || !filepath.IsLocal(rel) {
				return match
			}
			if hasAnchor {
				rel += "#" + anchor
			}
			return fields[1] + "(" + filepath.ToSlash(rel) + ")"
		}))
	}
	return strings.Join(parts, "\n\n"), nil
}
