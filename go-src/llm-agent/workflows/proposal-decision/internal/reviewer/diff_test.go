package reviewer

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"proposal-decision/internal/service"
	"proposal-platform/api"
)

func TestFileViewTreeDirectoriesAreArrays(t *testing.T) {
	for _, test := range []struct {
		path string
		want []string
	}{
		{"address_of.c", []string{}},
		{"src/pointers/address_of.c", []string{"src/pointers"}},
	} {
		t.Run(test.path, func(t *testing.T) {
			material := api.Candidate{
				Proposal: "fixture", ID: "candidate", Base: "base",
				Hunks: []api.Hunk{{
					ID: "hunk", Path: test.path, MustSign: true,
					Header: "@@ -1 +1 @@", OldStart: 1, NewStart: 1,
					Patch: "@@ -1 +1 @@\n-int x;\n+int *p;\n",
				}},
			}
			candidate, err := prepare(context.Background(), material, "")
			if err != nil {
				t.Fatal(err)
			}
			view := candidate.fileView(candidate.Files[test.path], service.ReviewRevision{}, service.ReviewSnapshot{}, nil)
			raw, err := json.Marshal(view)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Directories []string `json:"tree_directories"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wire.Directories, test.want) {
				t.Fatalf("tree_directories = %#v, want %#v; DiffTab requires an array", wire.Directories, test.want)
			}
		})
	}
}
