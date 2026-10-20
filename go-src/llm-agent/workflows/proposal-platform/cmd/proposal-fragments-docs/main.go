// proposal-fragments-docs extracts the proposal services' state, operations and
// loading/caching policies from Go declarations at one pinned Git commit.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type component struct{ Directory, State string }

var components = []component{
	{"proposal-platform/api", "Candidate"}, {"proposal-platform/journal", "Journal"}, {"proposal-platform/syncview", "Store"},
	{"proposal-decision/internal/candidate", "Builder"}, {"proposal-decision/internal/decision", "Revision"},
	{"proposal-decision/internal/service", "Service"}, {"proposal-decision/internal/reviewer", "Server"},
	{"proposal-workflow/internal/workflow", "Proposal"}, {"proposal-workflow/internal/service", "Service"},
	{"proposal-workflow/internal/ledger", "Ledger"}, {"proposal-workflow/internal/plan", "Service"},
}

const sourceRoot = "go-src/llm-agent/workflows/"

type source struct {
	Name string
	Tree *ast.File
}
type row struct{ Name, Type, Doc, Location string }
type documentation struct {
	Name, Doc, Loading, Caching string
	Fields, Actions             []row
}

func nodeText(fset *token.FileSet, node any) string {
	var b bytes.Buffer
	if err := format.Node(&b, fset, node); err != nil {
		panic(err)
	}
	return b.String()
}
func comment(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}
func policy(doc, name string) string {
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if text, ok := strings.CutPrefix(line, name+" policy:"); ok {
			parts := []string{strings.TrimSpace(text)}
			for _, tail := range lines[i+1:] {
				if strings.Contains(tail, " policy:") || strings.TrimSpace(tail) == "" {
					break
				}
				parts = append(parts, strings.TrimSpace(tail))
			}
			return strings.Join(parts, " ")
		}
	}
	return ""
}
func readSources(ctx context.Context, repo, oid string, fset *token.FileSet) ([]source, error) {
	args := []string{"-C", repo, "archive", "--format=tar", oid}
	for _, c := range components {
		args = append(args, sourceRoot+c.Directory)
	}
	command := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = command.Start(); err != nil {
		return nil, err
	}
	done := false
	defer func() {
		if !done {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	reader := tar.NewReader(pipe)
	files := []source{}
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if entry.Typeflag != tar.TypeReg || !strings.HasSuffix(entry.Name, ".go") || strings.HasSuffix(entry.Name, "_test.go") {
			continue
		}
		selected := false
		for _, c := range components {
			if path.Dir(entry.Name) == sourceRoot+c.Directory {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		if entry.Size > 2<<20 {
			return nil, fmt.Errorf("source too large: %s", entry.Name)
		}
		text, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		tree, err := parser.ParseFile(fset, entry.Name, text, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, source{entry.Name, tree})
	}
	err = command.Wait()
	done = true
	if err != nil {
		return nil, fmt.Errorf("read pinned Go source: %s", stderr.String())
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}
func extract(c component, files []source, fset *token.FileSet) (documentation, error) {
	doc := documentation{Name: c.Directory + "." + c.State}
	found := false
	operations := map[string]bool{}
	for _, file := range files {
		if path.Dir(file.Name) != sourceRoot+c.Directory {
			continue
		}
		for _, decl := range file.Tree.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					t, ok := spec.(*ast.TypeSpec)
					if !ok || t.Name.Name != c.State {
						continue
					}
					structure, ok := t.Type.(*ast.StructType)
					if !ok {
						return doc, fmt.Errorf("%s is not a struct", doc.Name)
					}
					if found {
						return doc, fmt.Errorf("duplicate %s", doc.Name)
					}
					found = true
					doc.Doc = comment(d.Doc)
					if t.Doc != nil {
						doc.Doc = comment(t.Doc)
					}
					doc.Loading = policy(doc.Doc, "Loading")
					doc.Caching = policy(doc.Doc, "Caching")
					for _, field := range structure.Fields.List {
						names := []string{}
						for _, name := range field.Names {
							names = append(names, name.Name)
						}
						if len(names) == 0 {
							names = []string{nodeText(fset, field.Type)}
						}
						for _, name := range names {
							doc.Fields = append(doc.Fields, row{name, nodeText(fset, field.Type), strings.TrimSpace(comment(field.Doc) + " " + comment(field.Comment)), fset.Position(field.Pos()).String()})
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv != nil && len(d.Recv.List) == 1 && strings.TrimPrefix(nodeText(fset, d.Recv.List[0].Type), "*") == c.State {
					doc.Actions = append(doc.Actions, row{d.Name.Name, nodeText(fset, d.Type), comment(d.Doc), fset.Position(d.Pos()).String()})
				}
				ast.Inspect(d.Body, func(n ast.Node) bool {
					sw, ok := n.(*ast.SwitchStmt)
					if !ok || sw.Tag == nil {
						return true
					}
					switch nodeText(fset, sw.Tag) {
					case "command.Action", "c.Action", "a.Kind", "operation", "action", "change.Kind":
					default:
						return true
					}
					for _, statement := range sw.Body.List {
						clause := statement.(*ast.CaseClause)
						for _, expr := range clause.List {
							literal, ok := expr.(*ast.BasicLit)
							if !ok || literal.Kind != token.STRING {
								continue
							}
							value, err := strconv.Unquote(literal.Value)
							if err != nil {
								continue
							}
							key := d.Name.Name + ":" + value
							if operations[key] {
								continue
							}
							operations[key] = true
							doc.Actions = append(doc.Actions, row{value, nodeText(fset, sw.Tag), "Dispatched by " + d.Name.Name, fset.Position(literal.Pos()).String()})
						}
					}
					return true
				})
			}
		}
	}
	if !found {
		return doc, fmt.Errorf("missing state declaration %s", doc.Name)
	}
	if doc.Loading == "" || doc.Caching == "" {
		return doc, fmt.Errorf("%s must document Loading policy and Caching policy beside its state", doc.Name)
	}
	return doc, nil
}

func code(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	for _, delimiter := range []string{"|", "!", ";", "+", "?", "~"} {
		if !strings.Contains(value, delimiter) {
			return "\\verb" + delimiter + value + delimiter
		}
	}
	// Split a rare mixed-operator spelling into independently delimited spans.
	parts := strings.Split(value, "|")
	for i := range parts {
		parts[i] = "\\verb|" + parts[i] + "|"
	}
	return strings.Join(parts, "\\verb!|!")
}
func text(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		for _, r := range word {
			if r < 128 && !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(".,;:!?'-/+", r)) {
				words[i] = code(word)
				break
			}
		}
	}
	return strings.Join(words, " ")
}
func render(docs []documentation, oid string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\\documentclass{article}\n\\kySourceDefaultOid{%s}\n\\begin{document}\n\\section*{拆分}\n\\begin{itemize}\n", oid)
	for _, doc := range docs {
		summary := strings.Split(doc.Doc, "\n")[0]
		fmt.Fprintf(&b, "\\item %s: %s\n", code(doc.Name), text(summary))
	}
	b.WriteString("\\end{itemize}\n")
	table := func(rows []row) {
		b.WriteString("\\begin{tabular}{ll}\n\\hline\n名称 & Go 定义与说明 \\\\\n\\hline\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "%s & %s %s %s \\\\\n", code(r.Name), code(r.Type), text(r.Doc), code(r.Location))
		}
		b.WriteString("\\hline\n\\end{tabular}\n")
	}
	for _, doc := range docs {
		fmt.Fprintf(&b, "\\section*{%s}\n\\textbf{数据字段}\n", text(doc.Name))
		table(doc.Fields)
		b.WriteString("\\textbf{动作与方法}\n")
		table(doc.Actions)
		fmt.Fprintf(&b, "\\textbf{加载策略}\n\n%s\n\n\\textbf{缓存策略}\n\n%s\n\n", text(doc.Loading), text(doc.Caching))
	}
	b.WriteString("\\end{document}\n")
	return b.String()
}
func run() error {
	repo := flag.String("repository", ".", "Go proposal source repository")
	output := flag.String("output", "", "generated document directory")
	oid := flag.String("oid", "", "source commit")
	flag.Parse()
	if *output == "" || *oid == "" || flag.NArg() != 0 {
		return fmt.Errorf("--output and --oid required; no positional arguments")
	}
	ctx := context.Background()
	commit, err := exec.CommandContext(ctx, "git", "-C", *repo, "rev-parse", "--verify", *oid+"^{commit}").Output()
	if err != nil {
		return err
	}
	*oid = strings.TrimSpace(string(commit))
	fset := token.NewFileSet()
	files, err := readSources(ctx, *repo, *oid, fset)
	if err != nil {
		return err
	}
	docs := []documentation{}
	for _, component := range components {
		doc, err := extract(component, files, fset)
		if err != nil {
			return err
		}
		docs = append(docs, doc)
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	file := filepath.Join(*output, "docs.tex")
	if err := os.WriteFile(file, []byte(render(docs, *oid)), 0644); err != nil {
		return err
	}
	fmt.Println("wrote", file)
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
