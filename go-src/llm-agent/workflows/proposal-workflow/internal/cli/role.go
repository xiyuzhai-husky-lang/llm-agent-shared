package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"proposal-workflow/internal/project"
)

func environment(name, value string) string {
	if actual := os.Getenv(name); actual != "" {
		return actual
	}
	return value
}

func invokeRole(ctx context.Context, prompt, output, directory, prefix, model, effort string) error {
	input, err := os.Open(prompt)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
		return err
	}
	log, err := os.Create(output + ".log")
	if err != nil {
		return err
	}
	defer log.Close()
	args := []string{"exec", "-C", directory, "-m", environment(prefix+"_MODEL", model), "-c", "model_reasoning_effort=" + environment(prefix+"_EFFORT", effort), "-c", "project_doc_max_bytes=0"}
	extra := strings.Split(environment(prefix+"_CODEX_ARGS", "--dangerously-bypass-approvals-and-sandbox"), "\n")[0]
	args = append(args, strings.FieldsFunc(extra, func(r rune) bool { return r == ' ' || r == '\t' })...)
	args = append(args, "--skip-git-repo-check", "-o", absolute(output))
	command := exec.CommandContext(ctx, "codex", args...)
	command.Stdin = input
	command.Stdout = log
	command.Stderr = log
	if err := command.Run(); err != nil {
		return fmt.Errorf("codex (log %s): %w", output+".log", err)
	}
	return nil
}

func commissarSubagent(ctx context.Context, args []string) error {
	if len(args) != 3 || args[0] != "commissar-subagent" {
		return fmt.Errorf("role commissar-subagent <prompt-file> <output-file>")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return invokeRole(ctx, args[1], args[2], environment("COMMISSAR_SUBAGENT_CWD", cwd), "COMMISSAR_SUBAGENT", "gpt-5.6-luna", "low")
}

func provost(ctx context.Context, o Options, args []string) error {
	if len(args) < 4 || args[0] != "provost" {
		return fmt.Errorf("document provost <document> <output-file> <YYYY-MM-DD ...>")
	}
	document, output := absolute(args[1]), absolute(args[2])
	if info, err := os.Stat(document); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("no such document: %s", document)
	}
	if o.Project == "" {
		return fmt.Errorf("--project required")
	}
	conversation := os.Getenv("PROVOST_CONVERSATION_DIR")
	if conversation == "" {
		key := environment("PROPOSAL_PROJECT_KEY", strings.TrimPrefix(o.Project, "projects/"))
		if !filepath.IsLocal(key) {
			return fmt.Errorf("invalid project key")
		}
		conversation = filepath.Join(o.Main, ".local/conversation", key)
	}
	for _, day := range args[3:] {
		if !regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`).MatchString(day) {
			return fmt.Errorf("invalid conversation day %q", day)
		}
		file, err := os.Open(filepath.Join(conversation, day+".md"))
		if err != nil {
			return err
		}
		file.Close()
	}
	charter, err := project.AssembleRole(o.Repository, o.Project, "provost")
	if err != nil {
		return err
	}
	if err := writeFile(output+".charter.md", []byte(charter)); err != nil {
		return err
	}
	template := environment("PROVOST_TEMPLATE", filepath.Join(o.Main, "projects/common/review/agents/provost-assignment-template.md"))
	data, err := os.ReadFile(template)
	if err != nil {
		return err
	}
	kind, prose := "document", "a document the orchestrator is about to send on"
	if strings.Contains(document, "assignment") {
		kind, prose = "assignment", "an assignment the orchestrator is about to send to a Worker"
	} else if strings.HasSuffix(document, ".ky.tex") || strings.Contains(document, "plan") {
		kind, prose = "page", "a page the orchestrator is about to put in front of the user"
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
	for _, pair := range [][2]string{{"KIND", kind}, {"KIND_PROSE", prose}, {"LABEL", filepath.Base(document)}, {"CHARTER", output + ".charter.md"}, {"DOCUMENT", document}, {"CONVERSATION_DIR", conversation}, {"DAYS", strings.Join(args[3:], " ")}, {"OUTPUT_PATH", output}} {
		text = strings.ReplaceAll(text, "{{"+pair[0]+"}}", pair[1])
	}
	if regexp.MustCompile(`\{\{[A-Z_]+\}\}`).MatchString(text) {
		return fmt.Errorf("unfilled provost assignment placeholder")
	}
	prompt := output + ".prompt.txt"
	if err := writeFile(prompt, []byte(text)); err != nil {
		return err
	}
	if err := invokeRole(ctx, prompt, output, o.Main, "PROVOST", "gpt-6-astra", "medium"); err != nil {
		return err
	}
	data, err = os.ReadFile(output)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Provost:") {
			fmt.Printf("provost report: %s\n%s\n", output, line)
			return nil
		}
	}
	return fmt.Errorf("provost report %s has no Provost: verdict line", output)
}
