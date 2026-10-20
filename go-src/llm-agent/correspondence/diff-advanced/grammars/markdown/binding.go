// Package markdown is the tree-sitter Markdown block grammar, compiled from the source `../fetch.sh` fetches into the
// gitignored `.local/diff-advanced/grammars/markdown/` at the commit `../manifest.json` pins; run `../fetch.sh` before
// building.
package markdown

// #cgo CFLAGS: -std=c11 -fPIC -w -I${SRCDIR}/../../../../../../.local/diff-advanced/grammars/markdown/tree-sitter-markdown/src
// #include "parser.c"
// #include "scanner.c"
import "C"

import "unsafe"

// Language is the grammar as a pointer for tree_sitter.NewLanguage.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_markdown())
}
