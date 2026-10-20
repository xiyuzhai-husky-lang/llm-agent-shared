package reviewer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"proposal-decision/internal/candidate"
	"proposal-platform/api"
)

// DiffByRefs prepares the standalone diff pages' complete immutable read model.
// It executes read-only Git commands once; subsequent file requests use this result.
func DiffByRefs(ctx context.Context, repository, base, revision string, working bool) (object, error) {
	resolve := func(ref string) (string, error) {
		data, err := readGit(ctx, repository, "rev-parse", "--verify", ref+"^{commit}").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("resolve %q: %s", ref, data)
		}
		return strings.TrimSpace(string(data)), nil
	}
	base, err := resolve(base)
	if err != nil {
		return nil, err
	}
	if !working {
		revision, err = resolve(revision)
		if err != nil {
			return nil, err
		}
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--binary", "--unified=3", base}
	if !working {
		args = append(args, revision)
	}
	args = append(args, "--")
	raw, err := readGit(ctx, repository, args...).Output()
	if err != nil {
		return nil, err
	}
	if len(raw) > 24<<20 {
		return nil, fmt.Errorf("diff exceeds 24 MiB")
	}
	if working {
		hash := sha256.Sum256(raw)
		revision = hex.EncodeToString(hash[:])
	}
	hunks, err := candidate.ParsePatch(string(raw), nil)
	if err != nil {
		return nil, err
	}
	c := &Candidate{Material: api.Candidate{ID: revision, Base: base, Hunks: hunks}, Files: map[string]*File{}, Sources: map[string]string{}}
	reads := map[string]bool{}
	for i, h := range hunks {
		h.ID = candidate.HunkID(h)
		c.Material.Hunks[i] = h
		if c.Files[h.Path] == nil {
			c.Files[h.Path] = &File{Path: h.Path}
			c.Paths = append(c.Paths, h.Path)
		}
		c.Files[h.Path].Hunks = append(c.Files[h.Path].Hunks, h)
		reads[base+":"+h.Path] = true
		if !working {
			reads[revision+":"+h.Path] = true
		}
	}
	c.Sources, err = readBlobs(ctx, repository, reads)
	if err != nil {
		return nil, err
	}
	if working {
		for _, p := range c.Paths {
			data, err := os.ReadFile(filepath.Join(repository, p))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			c.Sources[revision+":"+p] = string(data)
		}
	}
	for _, f := range c.Files {
		if err := f.prepare(ctx, c); err != nil {
			return nil, err
		}
	}
	diff := c.projectDiff()
	contents := map[string]string{}
	for _, p := range c.Paths {
		if text, ok := c.Sources[revision+":"+p]; ok {
			contents[p] = text
		} else {
			contents[p] = c.Sources[base+":"+p]
		}
	}
	side := object{"kind": "commit", "commit": revision}
	if working {
		side = object{"kind": "working-tree"}
	}
	result := diff.View
	result["base"] = base
	result["candidate"] = revision
	result["candidateSide"] = side
	result["baseShort"] = base[:11]
	result["candidateShort"] = revision[:11]
	result["patch"] = ""
	result["files"] = diff.Files
	result["contents"] = contents
	return result, nil
}
