package highlight

import (
	"path"
	"strings"
)

var extensions = map[string]string{
	"sh": "bash", "bash": "bash", "zsh": "bash", "c": "c", "h": "c", "cs": "c_sharp",
	"cpp": "cpp", "cc": "cpp", "cxx": "cpp", "hpp": "cpp", "hh": "cpp", "hxx": "cpp",
	"css": "css", "dart": "dart", "el": "elisp", "ex": "elixir", "exs": "elixir",
	"erb": "embedded_template", "ejs": "embedded_template", "go": "go", "html": "html", "htm": "html",
	"java": "java", "js": "javascript", "jsx": "javascript", "mjs": "javascript", "cjs": "javascript",
	"json": "json", "jsonc": "json", "kt": "kotlin", "kts": "kotlin", "lean": "lean",
	"tex": "latex", "sty": "latex", "cls": "latex", "lua": "lua", "m": "objc", "mm": "objc",
	"ml": "ocaml", "mli": "ocaml", "php": "php", "py": "python", "pyi": "python",
	"res": "rescript", "resi": "rescript", "rb": "ruby", "rs": "rust", "scala": "scala", "sc": "scala",
	"sol": "solidity", "swift": "swift", "rdl": "systemrdl", "tla": "tlaplus", "toml": "toml",
	"tsx": "tsx", "ts": "typescript", "mts": "typescript", "cts": "typescript", "vue": "vue",
	"yaml": "yaml", "yml": "yaml", "zig": "zig", "md": "markdown", "markdown": "markdown", "qcy": "scheme",
}

func Language(pathname string) string {
	name := strings.ToLower(path.Base(pathname))
	switch name {
	case ".bashrc", ".zshrc":
		return "bash"
	case "gemfile", "rakefile":
		return "ruby"
	}
	return extensions[strings.TrimPrefix(path.Ext(name), ".")]
}
