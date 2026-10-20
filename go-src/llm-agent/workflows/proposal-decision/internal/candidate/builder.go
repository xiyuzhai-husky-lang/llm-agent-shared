package candidate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"proposal-platform/api"
)

// Builder owns one-time candidate construction and observable heavy-operation counts.
// Loading policy: Build the pinned Git diff and highlight its content on candidate delivery.
// Caching policy: A built Candidate is immutable and is reused by every subsequent decision and view.
type Builder struct {
	Repository  Repository
	Highlighter Highlighter
	Builds      atomic.Uint64
	GitCalls    atomic.Uint64
}

func Label(value string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`).MatchString(value)
}

func HunkID(h api.Hunk) string {
	data, _ := json.Marshal([]string{h.Path, h.Header, h.Before, h.After})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (b *Builder) Build(ctx context.Context, input api.Candidate) (*Candidate, error) {
	b.Builds.Add(1)
	if !Label(input.Proposal) || !Label(input.ID) {
		return nil, fmt.Errorf("invalid proposal or candidate ID")
	}
	if input.Kind != "complete" && input.Kind != "incomplete" && input.Kind != "chow_problems" {
		return nil, fmt.Errorf("invalid candidate kind %q", input.Kind)
	}
	// The service takes ownership of decoded input; clone for callers using the Go API.
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	if len(encoded) > 24<<20 {
		return nil, fmt.Errorf("candidate exceeds 24 MiB")
	}
	var material api.Candidate
	if err = json.Unmarshal(encoded, &material); err != nil {
		return nil, err
	}
	if material.Source != nil {
		if len(material.Hunks) > 0 {
			return nil, fmt.Errorf("provide hunks or a Git source, not both")
		}
		if b.Repository == nil {
			return nil, fmt.Errorf("Git source unavailable")
		}
		if material.ID != material.Source.Commit || material.Base != material.Source.Base {
			return nil, fmt.Errorf("candidate identity must match its pinned Git range")
		}
		b.GitCalls.Add(1)
		patch, err := b.Repository.Patch(ctx, *material.Source)
		if err != nil {
			return nil, err
		}
		material.Hunks, err = ParsePatch(patch, material.Source.MustSign)
		if err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	for i, h := range material.Hunks {
		if h.Path == "" || strings.HasPrefix(h.Path, "/") || path.Clean(h.Path) != h.Path || h.Path == ".." || strings.HasPrefix(h.Path, "../") {
			return nil, fmt.Errorf("invalid hunk path %q", h.Path)
		}
		identity := HunkID(h)
		if h.ID != "" && h.ID != identity {
			return nil, fmt.Errorf("hunk ID does not match content")
		}
		if seen[identity] {
			return nil, fmt.Errorf("duplicate hunk")
		}
		seen[identity] = true
		material.Hunks[i].ID = identity
		if material.Source != nil {
			material.Hunks[i].Reason = material.Source.MustSignReasons[h.Path]
		}
	}
	sort.SliceStable(material.Hunks, func(i, j int) bool { return material.Hunks[i].MustSign && !material.Hunks[j].MustSign })
	doubts := map[string]bool{}
	for i, d := range material.Doubts {
		for index, anchor := range d.Anchors {
			if anchor.Header == "" {
				continue
			}
			found := ""
			for _, h := range material.Hunks {
				if matchesAnchor(h, anchor.Path, anchor.Header) {
					found = h.ID
					break
				}
			}
			if found == "" {
				return nil, fmt.Errorf("Doubt anchor does not match delivered diff: %s %s", anchor.Path, anchor.Header)
			}
			material.Doubts[i].Anchors[index].Hunk = found
		}
		if d.Hunk == "" && d.Path != "" && d.Header != "" {
			for _, h := range material.Hunks {
				if matchesAnchor(h, d.Path, d.Header) {
					d.Hunk = h.ID
					material.Doubts[i].Hunk = h.ID
					break
				}
			}
			if d.Hunk == "" {
				return nil, fmt.Errorf("Doubt anchor does not match the delivered diff: %s %s", d.Path, d.Header)
			}
		}
		if !Label(d.ID) || doubts[d.ID] || strings.TrimSpace(d.Text) == "" {
			return nil, fmt.Errorf("invalid or duplicate Doubt %q", d.ID)
		}
		doubts[d.ID] = true
		if d.Worker != "" && d.Worker != "agreed" && d.Worker != "disagreed" {
			return nil, fmt.Errorf("invalid Worker disposition %q", d.Worker)
		}
		if d.Hunk != "" && !seen[d.Hunk] {
			return nil, fmt.Errorf("unknown Doubt hunk %s", d.Hunk)
		}
	}
	docs := map[string]bool{}
	for _, d := range material.Documents {
		if !Label(d.ID) || docs[d.ID] {
			return nil, fmt.Errorf("invalid or duplicate document ID")
		}
		docs[d.ID] = true
	}
	if b.Highlighter != nil {
		if err := b.Highlighter.Highlight(ctx, material.Source, material.Hunks); err != nil {
			return nil, err
		}
	}
	return Restore(material), nil
}
