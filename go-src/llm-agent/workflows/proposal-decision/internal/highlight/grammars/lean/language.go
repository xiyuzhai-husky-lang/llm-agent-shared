package lean

//go:generate go run ../generate lean

// #cgo CFLAGS: -I.
// #include "tree_sitter/parser.h"
// const TSLanguage *tree_sitter_lean(void);
import "C"
import (
	_ "embed"
	"unsafe"
)

//go:embed highlights.scm
var Query string

func Language() unsafe.Pointer { return unsafe.Pointer(C.tree_sitter_lean()) }
