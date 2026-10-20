// Package document is the Kathy document, the intermediate representation a
// `.chw.tex` page converts to and the browser renders.
//
// TODO: the types follow webui/packages/kathy-ast/src/schema.generated.ts;
// only the top-level shape is written so far.
package document

// Document is one `.kathy.json`: an arena of nodes that the page's entry
// refers into by index.
type Document struct {
	Arena Arena `json:"arena"`
}

// Arena holds every node of the document, prose nodes in Rose and formula
// nodes in Math; a node refers to its children by index range.
type Arena struct {
	Math []Node `json:"math"`
	Rose []Node `json:"rose"`
}

// Node is one arena entry; Kind says which of the schema's node kinds it is
// and which other fields are present.
type Node struct {
	Kind string `json:"kind"`
}
