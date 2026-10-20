// Command generate restores native grammar sources for go generate. It uses
// only Go's standard library; no Rust, npm, or Tree-sitter CLI is required.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ../generate <lean|systemrdl>")
		os.Exit(1)
	}
	src, ok := sources[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "unknown grammar:", os.Args[1])
		os.Exit(1)
	}
	if err := generate(src, ".", &http.Client{Timeout: 2 * time.Minute}); err != nil {
		fmt.Fprintln(os.Stderr, os.Args[1]+":", err)
		os.Exit(1)
	}
}

func fileMatches(path, want string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return err == nil && fmt.Sprintf("%x", h.Sum(nil)) == want
}

func generate(src source, dest string, client *http.Client) error {
	current := true
	for _, file := range src.files {
		if !fileMatches(filepath.Join(dest, file.localPath), file.sha256) {
			current = false
			break
		}
	}
	if current {
		return nil
	}
	response, err := client.Get(src.url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", src.url, response.Status)
	}
	const maxArchive = 16 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxArchive+1))
	if err != nil {
		return err
	}
	if len(data) > maxArchive {
		return fmt.Errorf("archive exceeds %d bytes", maxArchive)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != src.sha256 {
		return fmt.Errorf("archive SHA-256 mismatch: %s", src.url)
	}
	// Validate every selected file before replacing any local output. Only the
	// fixed manifest determines output paths; other archive entries are ignored.
	stage, err := os.MkdirTemp(dest, ".grammar-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := extract(data, src.files, stage); err != nil {
		return err
	}
	for _, file := range src.files {
		path := filepath.Join(dest, file.localPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, file.localPath), path); err != nil {
			return err
		}
	}
	return nil
}

func extract(data []byte, files []sourceFile, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	archive := tar.NewReader(gz)
	selected := make(map[string]sourceFile, len(files))
	for _, file := range files {
		selected[file.archivePath] = file
	}
	seen := make(map[string]bool, len(files))
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		file, ok := selected[header.Name]
		if !ok {
			continue
		}
		if header.Typeflag != tar.TypeReg || seen[header.Name] || header.Size > 128<<20 {
			return fmt.Errorf("invalid archive entry: %s", header.Name)
		}
		seen[header.Name] = true
		path := filepath.Join(dest, file.localPath)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := writeFile(path, archive); err != nil {
			return err
		}
		if !fileMatches(path, file.sha256) {
			return fmt.Errorf("file SHA-256 mismatch: %s", header.Name)
		}
	}
	for _, file := range files {
		if !seen[file.archivePath] {
			return fmt.Errorf("missing archive entry: %s", file.archivePath)
		}
	}
	return nil
}

func writeFile(path string, contents io.Reader) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, contents)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
