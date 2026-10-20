; Minimal local query for the installed html grammar.
; No upstream highlights.scm was available in node_modules.
"<!" @operator
">" @operator
"doctype" @keyword
"<" @operator
"/>" @operator
"</" @operator
"=" @operator
(comment) @comment
(quoted_attribute_value) @string
(tag_name) @tag
(attribute_name) @attribute
