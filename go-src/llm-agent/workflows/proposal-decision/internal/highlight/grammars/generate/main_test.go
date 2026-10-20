package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testArchive(t *testing.T, names ...string) []byte {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		body := []byte("contents of " + name)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func testSource(data []byte) source {
	return source{sha256: fmt.Sprintf("%x", sha256.Sum256(data)), files: []sourceFile{
		{"release/parser.c", "parser.c", fmt.Sprintf("%x", sha256.Sum256([]byte("contents of release/parser.c")))},
		{"release/parser.h", "tree_sitter/parser.h", fmt.Sprintf("%x", sha256.Sum256([]byte("contents of release/parser.h")))},
	}}
}

func TestGenerateRestoresAndReusesExactFiles(t *testing.T) {
	data := testArchive(t, "release/parser.c", "release/parser.h", "release/lib.rs", "../outside")
	src := testSource(data)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(data)
	}))
	defer server.Close()
	src.url = server.URL
	dest := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := generate(src, dest, server.Client()); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 1 {
		t.Fatalf("unchanged outputs caused another download: %d requests", requests)
	}
	for _, file := range src.files {
		if !fileMatches(filepath.Join(dest, file.localPath), file.sha256) {
			t.Fatalf("output differs: %s", file.localPath)
		}
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected outputs: %v, %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dest, "parser.c"), []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := generate(src, dest, server.Client()); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !fileMatches(filepath.Join(dest, "parser.c"), src.files[0].sha256) {
		t.Fatal("corrupt local output was not repaired")
	}
	server.Close()
	if err := generate(src, dest, server.Client()); err != nil {
		t.Fatalf("verified outputs should work offline: %v", err)
	}
}

func TestGenerateRejectsInvalidReleasesBeforeReplacingOutputs(t *testing.T) {
	for _, kind := range []string{"archive hash", "file hash", "missing", "duplicate", "HTTP error"} {
		t.Run(kind, func(t *testing.T) {
			names := []string{"release/parser.c", "release/parser.h"}
			if kind == "missing" {
				names = names[:1]
			}
			if kind == "duplicate" {
				names = append(names, names[0])
			}
			data := testArchive(t, names...)
			src := testSource(data)
			if kind == "archive hash" {
				src.sha256 = "wrong"
			}
			if kind == "file hash" {
				src.files[1].sha256 = "wrong"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "HTTP error" {
					w.WriteHeader(http.StatusServiceUnavailable)
				}
				_, _ = w.Write(data)
			}))
			defer server.Close()
			src.url = server.URL
			dest := t.TempDir()
			path := filepath.Join(dest, "parser.c")
			if err := os.WriteFile(path, []byte("old output"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := generate(src, dest, server.Client()); err == nil {
				t.Fatal("accepted invalid release")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "old output" {
				t.Fatalf("failed generation replaced an output: %q, %v", got, err)
			}
			entries, err := os.ReadDir(dest)
			if err != nil || len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".grammar-") {
				t.Fatalf("failed generation left partial files: %v, %v", entries, err)
			}
		})
	}
}
