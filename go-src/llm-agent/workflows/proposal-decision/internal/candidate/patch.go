package candidate

import (
	"fmt"
	"proposal-platform/api"
	"regexp"
	"strconv"
	"strings"
)

var patchRange = regexp.MustCompile(`^@@ -([0-9]+)(?:,[0-9]+)? \+([0-9]+)(?:,[0-9]+)? @@`)

// parsePatch materializes textual and binary/metadata changes into server-owned hunks.
func ParsePatch(patch string, required []string) ([]api.Hunk, error) {
	hunks := []api.Hunk{}
	for _, section := range strings.Split("\n"+patch, "\ndiff --git ")[1:] {
		lines := strings.Split(strings.TrimSuffix(section, "\n"), "\n")
		name, err := patchPath(lines[0])
		if err != nil {
			return nil, err
		}
		must := false
		for _, prefix := range required {
			if name == prefix || strings.HasPrefix(name, strings.TrimSuffix(prefix, "/")+"/") {
				must = true
			}
		}
		start := -1
		for i, line := range lines {
			if strings.HasPrefix(line, "@@ ") {
				start = i
				break
			}
		}
		if start < 0 {
			hunks = append(hunks, api.Hunk{Path: name, Header: "File metadata or binary change", After: strings.Join(lines[1:], "\n"), MustSign: must})
			continue
		}
		metadata := []string{}
		for _, line := range lines[1:start] {
			if strings.HasPrefix(line, "old mode ") || strings.HasPrefix(line, "new mode ") || strings.HasPrefix(line, "new file mode ") || strings.HasPrefix(line, "deleted file mode ") {
				metadata = append(metadata, line)
			}
		}
		if len(metadata) > 0 {
			hunks = append(hunks, api.Hunk{Path: name, Header: "File metadata", After: strings.Join(metadata, "\n"), MustSign: must})
		}
		var current *api.Hunk
		var previous byte
		var before, after, patchLines []string
		flush := func() {
			if current != nil {
				current.Before = strings.Join(before, "")
				current.After = strings.Join(after, "")
				current.Patch = strings.Join(patchLines, "\n")
			}
		}
		for _, line := range lines[start:] {
			if strings.HasPrefix(line, "@@ ") {
				flush()
				before = nil
				after = nil
				previous = 0
				hunks = append(hunks, api.Hunk{Path: name, Header: line, MustSign: must})
				current = &hunks[len(hunks)-1]
				positions := patchRange.FindStringSubmatch(line)
				if positions == nil {
					return nil, fmt.Errorf("invalid hunk range %q", line)
				}
				current.OldStart, _ = strconv.Atoi(positions[1])
				current.NewStart, _ = strconv.Atoi(positions[2])
				patchLines = []string{line}
				continue
			}
			if current == nil || line == "" {
				continue
			}
			patchLines = append(patchLines, line)
			switch line[0] {
			case ' ':
				before = append(before, line[1:]+"\n")
				after = append(after, line[1:]+"\n")
			case '-':
				before = append(before, line[1:]+"\n")
			case '+':
				after = append(after, line[1:]+"\n")
			case '\\':
				if previous == '-' || previous == ' ' {
					before[len(before)-1] = strings.TrimSuffix(before[len(before)-1], "\n")
				}
				if previous == '+' || previous == ' ' {
					after[len(after)-1] = strings.TrimSuffix(after[len(after)-1], "\n")
				}
			default:
				return nil, fmt.Errorf("invalid patch line %q", line)
			}
			previous = line[0]
		}
		flush()
	}
	if strings.TrimSpace(patch) != "" && len(hunks) == 0 {
		return nil, fmt.Errorf("unrecognized Git patch")
	}
	return hunks, nil
}

func patchPath(header string) (string, error) {
	var source, destination string
	if strings.HasPrefix(header, `"`) {
		split := strings.Index(header, `" "`)
		if split < 0 {
			return "", fmt.Errorf("invalid quoted diff path")
		}
		var err error
		source, err = strconv.Unquote(header[:split+1])
		if err != nil {
			return "", err
		}
		destination, err = strconv.Unquote(header[split+2:])
		if err != nil {
			return "", err
		}
	} else {
		// --no-renames gives equal path names on both sides, even with embedded spaces.
		split := (len(header) - 1) / 2
		if split < 2 || header[split] != ' ' {
			return "", fmt.Errorf("invalid diff path")
		}
		source, destination = header[:split], header[split+1:]
	}
	if !strings.HasPrefix(source, "a/") || !strings.HasPrefix(destination, "b/") || source[2:] != destination[2:] {
		return "", fmt.Errorf("unexpected renamed or invalid diff path")
	}
	return destination[2:], nil
}
