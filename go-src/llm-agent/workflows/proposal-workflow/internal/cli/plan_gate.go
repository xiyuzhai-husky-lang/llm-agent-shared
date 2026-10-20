package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func planOneRuleGate(repository, instance string) error {
	failures := []string{}
	raw, err := os.ReadFile(filepath.Join(instance, "plan.chw.tex"))
	if err != nil {
		return err
	}
	source := string(raw)
	structural := regexp.MustCompile(`(?s)\\begin\{(lstlisting|verbatim)\}|\\verb(.)|\\%|%[^\n]*|\\section\*?\{([^{}]*)\}`)
	for {
		location := structural.FindStringSubmatchIndex(source)
		if location == nil {
			break
		}
		token := source[location[0]:location[1]]
		source = source[location[1]:]
		parts := structural.FindStringSubmatch(token)
		ending := ""
		if parts[1] != "" {
			ending = "\\end{" + parts[1] + "}"
		} else if parts[2] != "" {
			ending = parts[2]
		}
		if ending != "" {
			end := strings.Index(source, ending)
			if end < 0 {
				failures = append(failures, "unterminated literal in plan.chw.tex")
				break
			}
			source = source[end+len(ending):]
		} else if strings.HasPrefix(parts[3], "最小步骤") {
			failures = append(failures, "plan.chw.tex writes a 最小步骤 section; the change belongs in plan-change.json alone")
		}
	}
	raw, err = os.ReadFile(filepath.Join(instance, "plan-change.json"))
	if err != nil {
		return err
	}
	var change map[string]json.RawMessage
	if err := json.Unmarshal(raw, &change); err != nil {
		return err
	}
	if change != nil {
		var kind string
		if err := json.Unmarshal(change["kind"], &kind); err != nil {
			return err
		}
		fields := []string{}
		switch kind {
		case "other":
			fields = []string{"description"}
		case "add_command", "delete_command":
			fields = []string{"library_file", "command", "what_this_is", "why_this_is_needed"}
		case "modify_command":
			fields = []string{"library_file", "old_command", "new_command", "what_this_is", "why_this_is_needed"}
		default:
			return fmt.Errorf("invalid plan-change kind %q", kind)
		}
		values := map[string]string{}
		allowed := map[string]bool{"kind": true}
		for _, field := range fields {
			allowed[field] = true
			var value string
			if err := json.Unmarshal(change[field], &value); err != nil {
				return fmt.Errorf("plan-change.json %s: %w", field, err)
			}
			values[field] = value
		}
		for field := range change {
			if !allowed[field] {
				return fmt.Errorf("unknown plan-change field %s", field)
			}
		}
		if kind != "other" {
			library := values["library_file"]
			if !filepath.IsLocal(library) || !strings.HasPrefix(library, "library/") || (!strings.HasSuffix(library, ".qcy") && !strings.HasSuffix(library, ".qcy.tex")) {
				return fmt.Errorf("library_file must be a .qcy or .qcy.tex path under library/")
			}
			for _, field := range []string{"what_this_is", "why_this_is_needed"} {
				if strings.TrimSpace(values[field]) == "" {
					failures = append(failures, field+" is empty")
				}
			}
			commands := []string{values["command"]}
			if kind == "modify_command" {
				commands = []string{values["old_command"], values["new_command"]}
			}
			for _, command := range commands {
				command = strings.TrimSpace(command)
				one := false
				if strings.HasSuffix(library, ".qcy.tex") {
					one = strings.HasPrefix(command, `\begin{proposition}`) && strings.HasSuffix(command, `\end{proposition}`) && strings.Count(command, `\begin{proposition}`) == 1
				} else {
					depth, count := 0, 0
					for _, ch := range command {
						if ch == '(' {
							if depth == 0 {
								count++
							}
							depth++
						} else if ch == ')' && depth > 0 {
							depth--
						}
					}
					one = strings.HasPrefix(command, "(") && strings.HasSuffix(command, ")") && count == 1
				}
				if !one {
					failures = append(failures, "not exactly one command: "+command)
				}
			}
			if kind == "modify_command" || kind == "delete_command" {
				existing := values["command"]
				if kind == "modify_command" {
					existing = values["old_command"]
				}
				data, err := os.ReadFile(filepath.Join(repository, library))
				if err != nil {
					return err
				}
				if !bytes.Contains(data, []byte(strings.TrimSpace(existing))) {
					failures = append(failures, "command to change is absent from "+library)
				}
			}
		}
	}
	raw, err = os.ReadFile(filepath.Join(instance, "plan.kathy.json"))
	if err != nil {
		return err
	}
	if !json.Valid(raw) {
		failures = append(failures, "invalid plan.kathy.json")
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "\n"))
	}
	return nil
}
