; Minimal latex-lsp/tree-sitter-latex query, using only web-tree-sitter predicates.
[(line_comment) (block_comment) (comment_environment)] @comment

; Generic names and the grammar's specialized command nodes (e.g. \frac, \ref).
(command_name) @function
(_ command: _ @function)
(math_delimiter left_command: _ @function right_command: _ @function)

(begin command: _ @keyword name: (curly_group_text (text) @type))
(end command: _ @keyword name: (curly_group_text (text) @type))

((_ command: _ @keyword)
  (#any-of? @keyword "\\section" "\\section*" "\\subsection" "\\subsection*"
    "\\subsubsection" "\\subsubsection*" "\\chapter" "\\part" "\\paragraph"
    "\\documentclass" "\\usepackage" "\\newtheorem" "\\newtheorem*" "\\informal"))

(label) @label

; Keep math bodies plain: long .qcy.tex formulas remain readable, with commands
; and delimiters distinguished instead of painting the whole formula gold.
["$" "$$" "\\[" "\\]" "\\(" "\\)"] @string
["{" "}" "[" "]" (delimiter)] @punctuation
((command_name) @punctuation (#eq? @punctuation "\\\\"))
