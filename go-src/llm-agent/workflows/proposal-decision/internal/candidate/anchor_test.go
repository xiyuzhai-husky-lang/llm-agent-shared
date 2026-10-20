package candidate_test

import (
	"context"
	"proposal-decision/internal/candidate"
	"proposal-platform/api"
	"testing"
)

func TestAnchorsUseExactCoordinatesWithoutGitSectionLabel(t *testing.T) {
	h := api.Hunk{Path: "guide.md", Header: "@@ -25,7 +25,7 @@ Worker 先完成代码", Before: "old", After: "new", MustSign: true}
	input := api.Candidate{Proposal: "p", ID: "r1", Kind: "complete", Hunks: []api.Hunk{h}, Doubts: []api.Doubt{{ID: "d", Text: "concern", Path: h.Path, Header: "@@ -25,7 +25,7 @@", Anchors: []api.Anchor{{Path: h.Path, Header: "@@ -25,7 +25,7 @@"}}}}}
	built, err := (&candidate.Builder{}).Build(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	want := candidate.HunkID(h)
	if built.Material.Doubts[0].Hunk != want || built.Material.Doubts[0].Anchors[0].Hunk != want || built.Material.Hunks[0].Header != h.Header {
		t.Fatal("anchor changed hunk identity")
	}
	input.Doubts[0].Anchors[0].Header = "@@ -26,7 +26,7 @@"
	if _, err = (&candidate.Builder{}).Build(context.Background(), input); err == nil {
		t.Fatal("different coordinates matched")
	}
}
