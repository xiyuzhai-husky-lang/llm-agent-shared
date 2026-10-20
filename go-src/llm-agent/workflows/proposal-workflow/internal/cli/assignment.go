package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"proposal-workflow/internal/project"
	"regexp"
	"strings"
)

const scopeMarker = "This scope line is the orchestrator's reading of what the user asked for. It may be wrong: check it against his words below and in the conversation record, and protest if it does not match."

func userLines(text, stamp string, review bool) map[string]bool {
	entries := map[string]bool{}
	inside := false
	message := regexp.MustCompile(`^\[([^]]+)\] ([A-Z]+): ?(.*)$`)
	heading := regexp.MustCompile(`^### (\S+)(?: · (.*))?$`)
	for _, line := range strings.Split(text, "\n") {
		if match := message.FindStringSubmatch(line); match != nil {
			inside = match[2] == "USER" && (stamp == "" || stamp == match[1])
			if inside {
				entries[strings.TrimSpace(match[3])] = true
			}
		} else if match := heading.FindStringSubmatch(line); review && match != nil {
			inside = stamp == match[1] && !strings.Contains(match[2], "Worker ") && !strings.Contains(match[2], "ORCHESTRATOR")
		} else if review && strings.HasPrefix(line, "## ") {
			inside = false
		} else if inside && strings.TrimSpace(line) != "" {
			entries[strings.TrimSpace(line)] = true
		}
	}
	return entries
}

func CheckAssignment(o Options, path string) error {
	charter, err := project.AssembleRole(o.Repository, o.Project, "worker")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(o.Main, ".local/roles", strings.TrimPrefix(o.Project, "projects/"), "worker.md"), []byte(charter)); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(raw)
	conversation := filepath.Join(o.Main, ".local/conversation", strings.TrimPrefix(o.Project, "projects/"))
	head := regexp.MustCompile(`^\[(SCOPE|USER|MECHANICS|ORCHESTRATOR)(?:,\s*(.*?))?\]\s*$`)
	structured := false
	for _, line := range strings.Split(text, "\n") {
		if head.MatchString(line) {
			structured = true
			break
		}
	}
	problems := []string{}
	problem := func(line int, reason string) {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", path, line, reason))
	}
	if !structured {
		sources, err := filepath.Glob(filepath.Join(conversation, "*.md"))
		if err != nil {
			return err
		}
		records := map[string]bool{}
		for _, source := range sources {
			data, err := os.ReadFile(source)
			if err != nil {
				return err
			}
			for line := range userLines(string(data), "", false) {
				records[line] = true
			}
		}
		for i, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, ">") {
				continue
			}
			quote := strings.TrimSpace(strings.TrimLeft(line, ">"))
			if quote != "" && !records[quote] {
				problem(i+1, "user quote absent from conversation record: "+quote)
			}
		}
		if !strings.Contains(text, scopeMarker) {
			problem(1, "missing scope marker: "+scopeMarker)
		}
	} else {
		charter := filepath.Join(o.Main, ".local/roles", strings.TrimPrefix(o.Project, "projects/"), "worker.md")
		if _, err := os.Stat(charter); err != nil {
			problem(1, "missing assembled role document: "+charter)
		}
		confirmation := regexp.MustCompile(`confirmed by user (\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}) "(.+)"`)
		daySource := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\.md (\d{2}:\d{2})`)
		reviewSource := regexp.MustCompile(`(\S+\.md)\s+(\S+)`)
		restriction := regexp.MustCompile(`(?i)\b(no new|never|don't|do not|must not|not allowed|forbidden|only)\b|不许|不得|禁止|只能|不新增|不加|不改`)
		attribution := regexp.MustCompile(`\(per (USER|MECHANICS|SCOPE) above\)`)
		kind := ""
		quotes := map[string]bool{}
		seenScope, scopeHasMarker := false, false
		scopeLine := 0
		for i, line := range strings.Split(text, "\n") {
			if i == 0 && regexp.MustCompile(`^\[RANK (?:D|C|B|A|S|SS|SSS), deadline [^\]]+\]$`).MatchString(line) {
				continue
			}
			if match := head.FindStringSubmatch(line); match != nil {
				if kind == "SCOPE" && !scopeHasMarker {
					problem(scopeLine, "SCOPE missing marker: "+scopeMarker)
				}
				kind = match[1]
				argument := match[2]
				switch kind {
				case "SCOPE":
					seenScope = true
					scopeHasMarker = false
					scopeLine = i + 1
					parts := confirmation.FindStringSubmatch(argument)
					if parts == nil {
						problem(i+1, "SCOPE requires dated user confirmation")
						continue
					}
					source := filepath.Join(conversation, parts[1]+".md")
					data, err := os.ReadFile(source)
					if err != nil || !userLines(string(data), parts[2], false)[parts[3]] {
						problem(i+1, "SCOPE confirmation absent from conversation record")
					}
				case "USER":
					var source, stamp string
					review := false
					if parts := daySource.FindStringSubmatch(argument); parts != nil {
						source = filepath.Join(conversation, parts[1]+".md")
						stamp = parts[2]
					} else if parts := reviewSource.FindStringSubmatch(argument); parts != nil {
						source = parts[1]
						if !filepath.IsAbs(source) {
							source = filepath.Join(o.Repository, source)
						}
						stamp = parts[2]
						review = true
					} else {
						problem(i+1, "USER requires source path and timestamp")
						quotes = map[string]bool{}
						continue
					}
					data, err := os.ReadFile(source)
					if err != nil {
						problem(i+1, err.Error())
					}
					quotes = userLines(string(data), stamp, review)
				case "MECHANICS":
					source, _, ok := strings.Cut(argument, "§")
					source = strings.TrimSpace(source)
					if !filepath.IsAbs(source) {
						source = filepath.Join(o.Repository, source)
					}
					if _, err := os.Stat(source); err != nil || !ok {
						problem(i+1, "MECHANICS requires an existing path and section")
					}
				}
				continue
			}
			if strings.HasPrefix(line, "[") {
				kind = ""
				problem(i+1, "unknown assignment block")
				continue
			}
			trimmed := strings.TrimSpace(line)
			switch kind {
			case "SCOPE":
				scopeHasMarker = scopeHasMarker || strings.Contains(line, scopeMarker)
			case "USER":
				if trimmed != "" && !quotes[trimmed] {
					problem(i+1, "user quote absent from cited record: "+trimmed)
				}
			case "ORCHESTRATOR":
				for _, sentence := range regexp.MustCompile(`[.。;；]\s+`).Split(line, -1) {
					if restriction.MatchString(sentence) && !attribution.MatchString(sentence) {
						problem(i+1, "orchestrator restriction lacks a source: "+sentence)
					}
				}
			case "":
				if trimmed != "" && !strings.HasPrefix(line, "#") {
					problem(i+1, "text outside sourced block")
				}
			}
		}
		if kind == "SCOPE" && !scopeHasMarker {
			problem(scopeLine, "SCOPE missing marker: "+scopeMarker)
		}
		if !seenScope {
			problem(1, "no SCOPE block")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("assignment rejected:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}
