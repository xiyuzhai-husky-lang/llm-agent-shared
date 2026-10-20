package reviewer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Source answers explicit source openings at the delivered commit. Sources outside
// the initial document set are loaded once, on demand; no working tree is read.
func (s *Server) Source(ctx context.Context, sub Subject, p string) (string, error) {
	c, _, _, err := s.selection(sub)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(p) {
		roots := []string{s.Repository}
		if c.Material.Source != nil {
			roots = append(roots, c.Material.Source.Repository)
		}
		for _, root := range roots {
			if relative, err := filepath.Rel(root, p); err == nil && filepath.IsLocal(relative) {
				p = filepath.ToSlash(relative)
				break
			}
		}
	}
	if !filepath.IsLocal(p) || strings.ContainsAny(p, "\x00\r\n\\") {
		return "", fmt.Errorf("invalid repository path")
	}
	input := c.Material.ID + ":" + p
	if text, ok := c.Sources[input]; ok {
		return text, nil
	}
	cacheKey := c.Material.Proposal + "/" + input
	s.mu.RLock()
	text, ok := s.extraSources[cacheKey]
	s.mu.RUnlock()
	if ok {
		return text, nil
	}
	blobs, err := readBlobs(ctx, s.Repository, map[string]bool{input: true})
	if err != nil {
		return "", err
	}
	text, ok = blobs[input]
	if !ok {
		return "", fmt.Errorf("file does not exist at the candidate: %s", p)
	}
	s.mu.Lock()
	s.extraSources[cacheKey] = text
	s.mu.Unlock()
	return text, nil
}
