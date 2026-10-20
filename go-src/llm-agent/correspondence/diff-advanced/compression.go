package main

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// compressed gzips what `handler` answers when the browser accepts it, since a tunnel carries the built page; the
// synchrotron and Peek's stream are not compressed.
func compressed(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Vary", "Accept-Encoding")
		if !strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") || request.Header.Get("Range") != "" {
			handler.ServeHTTP(writer, request)
			return
		}
		zipped := &gzipResponseWriter{ResponseWriter: writer}
		defer zipped.close()
		handler.ServeHTTP(zipped, request)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
	plain       bool
}

func (zipped *gzipResponseWriter) WriteHeader(code int) {
	if zipped.wroteHeader {
		return
	}
	zipped.wroteHeader = true
	// A response with no body, such as Not Modified, is passed through untouched.
	if code == http.StatusNotModified || code == http.StatusNoContent || code < 200 {
		zipped.plain = true
	} else {
		zipped.Header().Del("Content-Length")
		zipped.Header().Set("Content-Encoding", "gzip")
		zipped.writer = gzip.NewWriter(zipped.ResponseWriter)
	}
	zipped.ResponseWriter.WriteHeader(code)
}

func (zipped *gzipResponseWriter) Write(data []byte) (int, error) {
	if !zipped.wroteHeader {
		zipped.WriteHeader(http.StatusOK)
	}
	if zipped.plain {
		return zipped.ResponseWriter.Write(data)
	}
	return zipped.writer.Write(data)
}

func (zipped *gzipResponseWriter) close() {
	if zipped.writer != nil {
		_ = zipped.writer.Close()
	}
}
