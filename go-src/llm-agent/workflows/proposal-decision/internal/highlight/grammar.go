package highlight

import (
	"fmt"
	"strings"
	"sync"
	"unsafe"

	bash "github.com/alexaandru/go-sitter-forest/bash"
	c "github.com/alexaandru/go-sitter-forest/c"
	csharp "github.com/alexaandru/go-sitter-forest/c_sharp"
	cpp "github.com/alexaandru/go-sitter-forest/cpp"
	css "github.com/alexaandru/go-sitter-forest/css"
	dart "github.com/alexaandru/go-sitter-forest/dart"
	elisp "github.com/alexaandru/go-sitter-forest/elisp"
	elixir "github.com/alexaandru/go-sitter-forest/elixir"
	embeddedtemplate "github.com/alexaandru/go-sitter-forest/embedded_template"
	golang "github.com/alexaandru/go-sitter-forest/go"
	html "github.com/alexaandru/go-sitter-forest/html"
	java "github.com/alexaandru/go-sitter-forest/java"
	javascript "github.com/alexaandru/go-sitter-forest/javascript"
	json "github.com/alexaandru/go-sitter-forest/json"
	kotlin "github.com/alexaandru/go-sitter-forest/kotlin"
	latex "github.com/alexaandru/go-sitter-forest/latex"
	lua "github.com/alexaandru/go-sitter-forest/lua"
	markdown "github.com/alexaandru/go-sitter-forest/markdown"
	markdowninline "github.com/alexaandru/go-sitter-forest/markdown_inline"
	objc "github.com/alexaandru/go-sitter-forest/objc"
	ocaml "github.com/alexaandru/go-sitter-forest/ocaml"
	php "github.com/alexaandru/go-sitter-forest/php"
	python "github.com/alexaandru/go-sitter-forest/python"
	rescript "github.com/alexaandru/go-sitter-forest/rescript"
	ruby "github.com/alexaandru/go-sitter-forest/ruby"
	rust "github.com/alexaandru/go-sitter-forest/rust"
	scala "github.com/alexaandru/go-sitter-forest/scala"
	scheme "github.com/alexaandru/go-sitter-forest/scheme"
	solidity "github.com/alexaandru/go-sitter-forest/solidity"
	swift "github.com/alexaandru/go-sitter-forest/swift"
	tlaplus "github.com/alexaandru/go-sitter-forest/tlaplus"
	toml "github.com/alexaandru/go-sitter-forest/toml"
	tsx "github.com/alexaandru/go-sitter-forest/tsx"
	typescript "github.com/alexaandru/go-sitter-forest/typescript"
	vue "github.com/alexaandru/go-sitter-forest/vue"
	yaml "github.com/alexaandru/go-sitter-forest/yaml"
	zig "github.com/alexaandru/go-sitter-forest/zig"
	sitter "github.com/tree-sitter/go-tree-sitter"

	"proposal-decision/internal/highlight/grammars/lean"
	"proposal-decision/internal/highlight/grammars/systemrdl"
)

// grammar owns an upstream parser and its lazily compiled, immutable query.
type grammar struct {
	language func() unsafe.Pointer
	query    string
	once     sync.Once
	compiled *sitter.Query
	err      error
}

var grammars = map[string]*grammar{
	"bash":              {language: bash.GetLanguage, query: string(bash.GetQuery("highlights", bash.NativeFirst))},
	"c":                 {language: c.GetLanguage, query: string(c.GetQuery("highlights", c.NativeFirst))},
	"c_sharp":           {language: csharp.GetLanguage, query: string(csharp.GetQuery("highlights", csharp.NativeFirst))},
	"cpp":               {language: cpp.GetLanguage, query: string(cpp.GetQuery("highlights", cpp.NativeFirst))},
	"css":               {language: css.GetLanguage, query: string(css.GetQuery("highlights", css.NativeFirst))},
	"dart":              {language: dart.GetLanguage, query: string(dart.GetQuery("highlights", dart.NativeFirst))},
	"elisp":             {language: elisp.GetLanguage, query: string(elisp.GetQuery("highlights", elisp.NativeFirst))},
	"elixir":            {language: elixir.GetLanguage, query: string(elixir.GetQuery("highlights", elixir.NativeFirst))},
	"embedded_template": {language: embeddedtemplate.GetLanguage, query: string(embeddedtemplate.GetQuery("highlights", embeddedtemplate.NativeFirst))},
	"go":                {language: golang.GetLanguage, query: string(golang.GetQuery("highlights", golang.NativeFirst))},
	"html":              {language: html.GetLanguage, query: string(html.GetQuery("highlights", html.NativeFirst))},
	"java":              {language: java.GetLanguage, query: string(java.GetQuery("highlights", java.NativeFirst))},
	"javascript":        {language: javascript.GetLanguage, query: string(javascript.GetQuery("highlights", javascript.NativeFirst))},
	"json":              {language: json.GetLanguage, query: string(json.GetQuery("highlights", json.NativeFirst))},
	"kotlin":            {language: kotlin.GetLanguage, query: string(kotlin.GetQuery("highlights", kotlin.NativeFirst))},
	"latex":             {language: latex.GetLanguage, query: string(latex.GetQuery("highlights", latex.NativeFirst))},
	"lua":               {language: lua.GetLanguage, query: string(lua.GetQuery("highlights", lua.NativeFirst))},
	"objc":              {language: objc.GetLanguage, query: string(objc.GetQuery("highlights", objc.NativeFirst))},
	"ocaml":             {language: ocaml.GetLanguage, query: string(ocaml.GetQuery("highlights", ocaml.NativeFirst))},
	"php":               {language: php.GetLanguage, query: string(php.GetQuery("highlights", php.NativeFirst))},
	"python":            {language: python.GetLanguage, query: string(python.GetQuery("highlights", python.NativeFirst))},
	"rescript":          {language: rescript.GetLanguage, query: string(rescript.GetQuery("highlights", rescript.NativeFirst))},
	"ruby":              {language: ruby.GetLanguage, query: string(ruby.GetQuery("highlights", ruby.NativeFirst))},
	"rust":              {language: rust.GetLanguage, query: string(rust.GetQuery("highlights", rust.NativeFirst))},
	"scala":             {language: scala.GetLanguage, query: string(scala.GetQuery("highlights", scala.NativeFirst))},
	"solidity":          {language: solidity.GetLanguage, query: string(solidity.GetQuery("highlights", solidity.NativeFirst))},
	"swift":             {language: swift.GetLanguage, query: string(swift.GetQuery("highlights", swift.NativeFirst))},
	"tlaplus":           {language: tlaplus.GetLanguage, query: string(tlaplus.GetQuery("highlights", tlaplus.NativeFirst))},
	"toml":              {language: toml.GetLanguage, query: string(toml.GetQuery("highlights", toml.NativeFirst))},
	"tsx":               {language: tsx.GetLanguage, query: string(tsx.GetQuery("highlights", tsx.NativeFirst))},
	"typescript":        {language: typescript.GetLanguage, query: string(typescript.GetQuery("highlights", typescript.NativeFirst))},
	"vue":               {language: vue.GetLanguage, query: string(vue.GetQuery("highlights", vue.NativeFirst))},
	"yaml":              {language: yaml.GetLanguage, query: string(yaml.GetQuery("highlights", yaml.NativeFirst))},
	"zig":               {language: zig.GetLanguage, query: string(zig.GetQuery("highlights", zig.NativeFirst))},
	"markdown":          {language: markdown.GetLanguage, query: string(markdown.GetQuery("highlights", markdown.NativeFirst))},
	"markdown_inline":   {language: markdowninline.GetLanguage, query: string(markdowninline.GetQuery("highlights", markdowninline.NativeFirst))},
	"scheme":            {language: scheme.GetLanguage, query: string(scheme.GetQuery("highlights", scheme.NativeFirst))},
	"lean":              {language: lean.Language, query: lean.Query},
	"systemrdl":         {language: systemrdl.Language, query: systemrdl.Query},
}

// These grammars extend a parent grammar; upstream ships their queries separately.
var parents = map[string][]string{
	"cpp": {"c"}, "objc": {"c"}, "typescript": {"javascript"},
	"tsx": {"javascript"}, "vue": {"html"},
}

func queryText(name string) string { return queryTextWith(name, grammars[name].query) }
func queryTextWith(name, query string) string {
	text := ""
	for _, parent := range parents[name] {
		text += queryText(parent) + "\n"
	}
	// Dart 1.9.4 parses dynamic as an identifier, not an anonymous keyword.
	if name == "dart" {
		query = strings.ReplaceAll(query, `    "dynamic"`, "")
	}
	// This Neovim directive associates a URL with a capture, not a syntax color.
	if name == "latex" {
		query = strings.ReplaceAll(query, `(#set! @_hyperlink url @markup.link.url)`, "")
	}
	return text + query
}

func (g *grammar) compile(name string) (*sitter.Query, error) {
	g.once.Do(func() {
		if g.query == "" {
			g.err = fmt.Errorf("missing %s highlight query", name)
			return
		}
		query, err := sitter.NewQuery(sitter.NewLanguage(g.language()), queryTextWith(name, g.query))
		if err != nil {
			g.err = err
			return
		}
		g.compiled = query
	})
	return g.compiled, g.err
}
