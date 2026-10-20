; Minimal local query for the installed yaml grammar.
; No upstream highlights.scm was available in node_modules.
"---" @operator
"..." @punctuation.delimiter
"-" @operator
"?" @operator
":" @punctuation.delimiter
"|" @operator
">" @operator
"[" @punctuation.delimiter
"]" @punctuation.delimiter
"{" @punctuation.delimiter
"}" @punctuation.delimiter
"," @punctuation.delimiter
(boolean_scalar) @number
(float_scalar) @number
(integer_scalar) @number
(null_scalar) @number
"&" @operator
"*" @operator
(comment) @comment
(double_quote_scalar) @string
(single_quote_scalar) @string
(plain_scalar) @variable
(string_scalar) @string
(block_scalar) @string
(block_mapping_pair key: (flow_node) @property)
(block_mapping_pair key: (flow_node (plain_scalar (string_scalar) @property)))
