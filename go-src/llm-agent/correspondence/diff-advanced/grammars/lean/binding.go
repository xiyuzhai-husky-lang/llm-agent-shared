// Package lean is the tree-sitter Lean 4 grammar, compiled from the source `../fetch.sh` fetches into the
// gitignored `.local/diff-advanced/grammars/lean/` at the commit `../manifest.json` pins; run `../fetch.sh` before
// building.
package lean

// #cgo CFLAGS: -std=c11 -fPIC -w -I${SRCDIR}/../../../../../../.local/diff-advanced/grammars/lean/src
// #include "parser.c"
// #include "scanner.c"
import "C"

import "unsafe"

// Language is the grammar as a pointer for tree_sitter.NewLanguage.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_lean())
}
