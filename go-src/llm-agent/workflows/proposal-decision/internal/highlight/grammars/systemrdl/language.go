package systemrdl

//go:generate go run ../generate systemrdl

// #cgo CFLAGS: -I.
// #include "tree_sitter/parser.h"
// const TSLanguage *tree_sitter_systemrdl(void);
import "C"
import (
	_ "embed"
	"unsafe"
)

//go:embed highlights.scm
var Query string

func Language() unsafe.Pointer { return unsafe.Pointer(C.tree_sitter_systemrdl()) }
