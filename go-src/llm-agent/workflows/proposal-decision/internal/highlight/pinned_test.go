package highlight

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateHighlightGolden = flag.Bool("update-highlight-golden", false, "update pinned grammar HTML fixtures")

// These fixtures were captured with the original vendored parsers. Check both
// delivered hunk HTML and the reviewer's separate token vocabulary byte for byte.
func TestPinnedGrammarHTML(t *testing.T) {
	for _, name := range []string{"pinned.lean", "pinned.rdl"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"hunk", "reviewer"} {
				t.Run(mode, func(t *testing.T) {
					queries, classify := grammars, captureClass
					if mode == "reviewer" {
						queries, classify = reviewerGrammars, reviewerClass
					}
					doc, err := parseWith(context.Background(), Language(name), string(source), queries, classify)
					if err != nil {
						t.Fatal(err)
					}
					// Render each line with its full-file parse context, as a hunk
					// may start inside a multiline or nested comment.
					var rendered strings.Builder
					for i, line := range strings.SplitAfter(string(source), "\n") {
						html, err := doc.render(i+1, line)
						if err != nil {
							t.Fatal(err)
						}
						rendered.WriteString(html)
					}
					golden := path + "." + mode + ".html"
					if *updateHighlightGolden {
						if err := os.WriteFile(golden, []byte(rendered.String()), 0644); err != nil {
							t.Fatal(err)
						}
					}
					want, err := os.ReadFile(golden)
					if err != nil {
						t.Fatal(err)
					}
					if rendered.String() != string(want) {
						t.Fatalf("highlight HTML changed from %s\ngot: %s", golden, rendered.String())
					}
				})
			}
		})
	}
}
