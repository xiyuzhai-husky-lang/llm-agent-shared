; Minimal local query for the installed toml grammar.
; No upstream highlights.scm was available in node_modules.
(comment) @comment
"[" @punctuation.delimiter
"]" @punctuation.delimiter
"[[" @punctuation.delimiter
"]]" @punctuation.delimiter
"=" @operator
"." @punctuation.delimiter
(boolean) @number
"," @punctuation.delimiter
"{" @punctuation.delimiter
"}" @punctuation.delimiter
(string) @string
(integer) @number
(float) @number
(bare_key) @property
(quoted_key) @property
