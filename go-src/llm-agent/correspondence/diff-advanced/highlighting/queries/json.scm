; Minimal local query for the installed json grammar.
; No upstream highlights.scm was available in node_modules.
"{" @punctuation.delimiter
"," @punctuation.delimiter
"}" @punctuation.delimiter
":" @punctuation.delimiter
"[" @punctuation.delimiter
"]" @punctuation.delimiter
(number) @number
(true) @number
(false) @number
(null) @number
(comment) @comment
(string) @string
(string_content) @string
(pair key: (string) @property)
(pair key: (string (string_content) @property))
