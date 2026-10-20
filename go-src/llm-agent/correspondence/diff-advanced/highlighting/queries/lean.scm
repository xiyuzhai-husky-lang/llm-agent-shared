; The highlights query `arborium-lean` bundles, with one pattern moved: the catch-all `(identifier) @variable` is
; first here and sits in the middle there. The Diff view resolves two captures of one node in favour of the later
; capture, so a catch-all placed after the specific patterns defeats every one of them and paints each definition,
; type, constructor, field and call name as a plain variable. Every browser query in
; `projects/common/diff/ui/src/treeSitter/queries/` puts its catch-all first for exactly this reason; the bundled
; query is written for a highlighter that prefers the earlier pattern instead. Nothing else is changed, and
; `the_lean_query_is_the_bundled_query_with_its_catch_all_moved_first` checks that.

; Identifiers
(identifier) @variable

; Lean 4 highlights query

; Comments
(comment) @comment

; Strings
(string) @string
(char) @character
(interpolated_string) @string
(interpolation) @punctuation.special
(quoted_char) @character

; Numbers
(number) @number
(float) @number

; Boolean literals
(true) @constant.builtin
(false) @constant.builtin

; Special values
(sorry) @warning
(hole) @punctuation.special
(synthetic_hole) @punctuation.special
(cdot) @punctuation.special

; Definitions
(def
  (identifier) @function.definition)

(theorem
  (identifier) @function.definition)

(axiom
  (identifier) @function.definition)

(example
  (identifier) @function.definition)

(abbrev
  (identifier) @function.definition)

(constant
  (identifier) @constant)

; Type definitions
(inductive
  (identifier) @type.definition)

(class_inductive
  (identifier) @type.definition)

(structure
  (identifier) @type.definition)

; Constructor
(constructor
  (identifier) @constructor)

; Field access
(field
  (identifier) @property)

(proj
  (identifier) @property)

; Function application
(apply
  (identifier) @function.call)


; Quoted names
(quoted_name) @string.special
(double_quoted_name) @string.special

; Keywords
[
  "def"
  "theorem"
  "axiom"
  "example"
  "abbrev"
  "constant"
  "inductive"
  "class"
  "instance"
  "structure"
  "where"
  "extends"
  "deriving"
] @keyword

; Control flow
[
  "if"
  "then"
  "else"
  "match"
  "with"
  "do"
  "return"
  "for"
  "in"
  "unless"
] @keyword.conditional

; Binding keywords
[
  "let"
  "have"
  "fun"
  "λ"
] @keyword.function

; Namespace/module keywords
[
  "namespace"
  "section"
  "end"
  "open"
  "export"
  "import"
] @keyword.import

; Visibility/modifiers
[
  "private"
  "protected"
  "partial"
  "unsafe"
  "noncomputable"
] @keyword.modifier

; Variable declaration
[
  "variable"
  "universe"
] @keyword

; Tactics
(tactics) @function.builtin
(apply_tactic) @function.builtin
(intro) @function.builtin
(simp) @function.builtin
(rewrite) @function.builtin
(trivial) @function.builtin
(rfl) @function.builtin
(assumption_literal) @function.builtin

; Type annotations
(type_ascription
  ":" @punctuation.delimiter)

; Arrows and operators
[
  "→"
  "->"
  "←"
  "<-"
  "=>"
  "↔"
  ":="
  "="
  "≠"
  "<"
  ">"
  "≤"
  "<="
  "≥"
  ">="
  "+"
  "-"
  "*"
  "/"
  "%"
  "^"
  "∧"
  "∨"
  "¬"
  "∀"
  "∃"
  "λ"
  "|"
  "."
  "·"
] @operator

; Punctuation
[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
  "⟨"
  "⟩"
] @punctuation.bracket

[
  ":"
  ","
  ";"
] @punctuation.delimiter

; Attributes
(attribute) @attribute
(attributes) @attribute

; Hash commands
(hash_command) @keyword
