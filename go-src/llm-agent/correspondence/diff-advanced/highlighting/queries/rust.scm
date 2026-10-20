; The browser producer's own rust.scm, `projects/common/diff/ui/src/treeSitter/queries/rust.scm`, with one pattern
; removed: `"~" @operator`. The browser parses with the tree-sitter-wasms build of tree-sitter-rust 0.20, whose
; grammar still carried the anonymous `~` token; the tree-sitter-rust crate this workspace pins dropped it, and a
; query naming a node type the grammar does not have does not compile. No Rust source contains a `~` token, so the
; pattern matched nothing in the browser either and the painted tokens are the same. Every other pattern is verbatim,
; and `the_rust_query_is_the_browser_query_without_its_one_dead_pattern` checks that.
; Minimal local query for the installed rust grammar.
; No upstream highlights.scm was available in node_modules.
(identifier) @variable
";" @punctuation.delimiter
"(" @punctuation.delimiter
")" @punctuation.delimiter
"{" @punctuation.delimiter
"}" @punctuation.delimiter
"=>" @operator
"[" @punctuation.delimiter
"]" @punctuation.delimiter
":" @punctuation.delimiter
"+" @operator
"*" @operator
"?" @operator
"block" @keyword
"expr" @keyword
"ident" @keyword
"item" @keyword
"lifetime" @keyword
"literal" @keyword
"meta" @keyword
"pat" @keyword
"path" @keyword
"stmt" @keyword
"tt" @keyword
"ty" @keyword
"vis" @keyword
(primitive_type) @type
"/" @operator
"-" @operator
"=" @operator
"->" @operator
"," @punctuation.delimiter
"::" @punctuation.delimiter
"!" @operator
"." @punctuation.delimiter
"&" @operator
"%" @operator
"^" @operator
"<" @operator
">" @operator
"|" @operator
"as" @keyword
"async" @keyword
"await" @keyword
"break" @keyword
"const" @keyword
"continue" @keyword
"default" @keyword
"enum" @keyword
"fn" @keyword
"for" @keyword
"if" @keyword
"impl" @keyword
"let" @keyword
"loop" @keyword
"match" @keyword
"mod" @keyword
"pub" @keyword
"return" @keyword
"static" @keyword
"struct" @keyword
"trait" @keyword
"type" @keyword
"union" @keyword
"unsafe" @keyword
"use" @keyword
"where" @keyword
"while" @keyword
"extern" @keyword
"ref" @keyword
"else" @keyword
"..." @punctuation.delimiter
"in" @keyword
"dyn" @keyword
".." @punctuation.delimiter
"&&" @operator
"||" @operator
"==" @operator
"!=" @operator
"<=" @operator
">=" @operator
"<<" @operator
">>" @operator
"+=" @operator
"-=" @operator
"*=" @operator
"/=" @operator
"%=" @operator
"&=" @operator
"|=" @operator
"^=" @operator
"<<=" @operator
">>=" @operator
"yield" @keyword
"move" @keyword
(integer_literal) @number
(char_literal) @string
"true" @keyword
"false" @keyword
(line_comment) @comment
(raw_string_literal) @string
(float_literal) @number
(block_comment) @comment
(string_literal) @string
(field_identifier) @property
(type_identifier) @type
(function_item name: (identifier) @function)
(function_signature_item name: (identifier) @function)
(call_expression function: (identifier) @function)
