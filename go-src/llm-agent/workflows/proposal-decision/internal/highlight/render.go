package highlight

import (
	"context"
	_ "embed"
	"strings"
)

//go:embed reviewer_queries/lean.scm
var reviewerLean string

var reviewerGrammars = func() map[string]*grammar {
	result := map[string]*grammar{}
	for name, g := range grammars {
		result[name] = g
	}
	result["lean"] = &grammar{language: grammars["lean"].language, query: reviewerLean}
	return result
}()

// RenderFile prepares whole-file syntax in the shared reviewer's token vocabulary.
// Both this renderer and delivered hunk HTML use the same native parsers.
func RenderFile(ctx context.Context, path, text string) (string, error) {
	d, err := parseWith(ctx, Language(path), text, reviewerGrammars, reviewerClass)
	if err != nil {
		return "", err
	}
	return d.render(1, text)
}
func reviewerClass(name string) string {
	switch name {
	case "text.emphasis", "text.strong", "none":
		return ""
	case "variable.parameter":
		return "Parameter"
	case "function.method", "method":
		return "MethodEntity"
	case "constructor", "type", "type.definition":
		return "TypeEntity"
	case "text.title":
		return "DefEntity"
	case "text.literal", "text.uri", "string", "string.special", "string.escape", "number", "boolean", "character":
		return "Literal"
	case "text.reference", "label":
		return "Label"
	case "comment":
		return "Comment"
	case "warning":
		return "Todo"
	case "constant", "constant.builtin":
		return "ValEntity"
	case "function", "function.builtin", "function.call", "function.definition":
		return "FunctionEntity"
	case "variable":
		return "Variable"
	case "property":
		return "Field"
	case "tag":
		return "HtmlTagKind"
	case "attribute":
		return "Attribute"
	case "namespace":
		return "ModuleEntity"
	}
	if strings.HasPrefix(name, "keyword") {
		return "OtherKeyword"
	}
	if strings.HasPrefix(name, "punctuation") || name == "operator" {
		return "Punctuation"
	}
	return map[string]string{"keyword": "OtherKeyword", "function": "FunctionEntity", "type": "TypeEntity", "string": "Literal", "constant": "Literal", "comment": "Comment", "punctuation": "Punctuation", "variable": "Variable", "tag": "HtmlTagKind", "markup": "Label"}[captureClass(name)]
}
