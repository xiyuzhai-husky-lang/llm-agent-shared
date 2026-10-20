package main

type source struct {
	url    string
	sha256 string
	files  []sourceFile
}

type sourceFile struct {
	archivePath string
	localPath   string
	sha256      string
}

// Pin the exact release artifacts used by the original vendored grammars.
// File hashes also let repeated generation verify local outputs without network.
var sources = map[string]source{
	"lean": {
		url:    "https://static.crates.io/crates/arborium-lean/arborium-lean-2.18.2.crate",
		sha256: "02e3e3af99597613fc97e63696fdec5020506ffd9849bc4172b8e01056afa936",
		files: []sourceFile{
			{"arborium-lean-2.18.2/grammar/src/parser.c", "parser.c", "1797a7f6375e149624591ee1a5ba79d98ce58f810a955ba3cb3ba1fc4753cdf5"},
			{"arborium-lean-2.18.2/grammar/scanner.c", "scanner.c", "4a6a667ff32debf06dab40f1b70c3692bf377a13c17545bfb78cc82a45d4be74"},
			{"arborium-lean-2.18.2/grammar/src/tree_sitter/alloc.h", "tree_sitter/alloc.h", "b29c1c9fb7cc82f58c84b376df1297d6e2737a1d655fd356db0859e3c29c2fea"},
			{"arborium-lean-2.18.2/grammar/src/tree_sitter/array.h", "tree_sitter/array.h", "31e60a1bff6f715afacce03b5b70efe42b58371b4f9595dd4af52a577ff9608c"},
			{"arborium-lean-2.18.2/grammar/src/tree_sitter/parser.h", "tree_sitter/parser.h", "180b893c8734778fd32f372dfbc27bd6ad1cd2221f26150b31256ff6716320d2"},
		},
	},
	"systemrdl": {
		url:    "https://registry.npmjs.org/tree-sitter-systemrdl/-/tree-sitter-systemrdl-0.8.0.tgz",
		sha256: "90406486761e90f9df407eca270890edfd8512a35cf5158db3838e7f3cbc72f5",
		files: []sourceFile{
			{"package/src/parser.c", "parser.c", "2486ea5b09bee191c8e409b92ba71d1b3348fb0f1ef8f1f723bb6a5cab7ccb94"},
			{"package/src/tree_sitter/parser.h", "tree_sitter/parser.h", "c569eeb34b7ed816b3a0a43c4cb8c06b36a6ef1164bc82ebed5047284cd73036"},
		},
	},
}
