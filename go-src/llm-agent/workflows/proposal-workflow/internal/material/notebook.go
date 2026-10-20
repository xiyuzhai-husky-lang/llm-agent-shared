package material

import (
	"context"
	"fmt"
	"html"
	"strings"

	"proposal-platform/notebook"
	"proposal-workflow/internal/workflow"
)

// Notebook validates pinned source ranges and preserves each item's independent discussion.
func (b Builder) Notebook(ctx context.Context, t workflow.Task, oid, text string) (string, error) {
	document, err := notebook.Parse(t.Work.Folder, text)
	if err != nil {
		return "", err
	}
	if err := document.Bind(oid, func(revision, path string) (string, error) {
		return b.Repository.Read(ctx, t.Directory, revision, path)
	}); err != nil {
		return "", err
	}
	var output strings.Builder
	for _, group := range document.Groups {
		fmt.Fprintf(&output, "<h2>%s</h2>", html.EscapeString(group.Title))
		for _, item := range group.Items {
			fmt.Fprintf(&output, "<article><h3>%s</h3><p>%s · %s</p><pre>", html.EscapeString(item.Title), html.EscapeString(item.Source.Path), html.EscapeString(item.RangeLabel))
			for _, line := range item.Lines {
				if line.Quoted {
					fmt.Fprintln(&output, html.EscapeString(line.Text))
				}
			}
			fmt.Fprintf(&output, "</pre><p>%s</p></article>", html.EscapeString(item.Discussion))
		}
	}
	return output.String(), nil
}
