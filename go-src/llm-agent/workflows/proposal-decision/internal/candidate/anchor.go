package candidate

import (
	"proposal-platform/api"
	"regexp"
)

var hunkRange = regexp.MustCompile(`^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)(?: .*)?$`)

// Review anchors name the coordinate range; Git may append a section/function
// label. That label remains in the immutable hunk and its signature identity.
func matchesAnchor(h api.Hunk, path, header string) bool {
	if h.Path != path {
		return false
	}
	left, right := hunkRange.FindStringSubmatch(h.Header), hunkRange.FindStringSubmatch(header)
	return len(left) > 1 && len(right) > 1 && left[1] == right[1]
}
