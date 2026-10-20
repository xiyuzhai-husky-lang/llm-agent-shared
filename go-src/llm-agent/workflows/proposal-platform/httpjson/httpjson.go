// Package httpjson provides strict, bounded JSON and same-origin mutation handling.
package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func Decode(w http.ResponseWriter, r *http.Request, target any) error {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			return errors.New("cross-origin command refused")
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}

func Write(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "response encoding failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func Error(w http.ResponseWriter, err error) {
	Write(w, http.StatusConflict, map[string]string{"error": err.Error()})
}

func Cursor(r *http.Request) (uint64, error) {
	if r.URL.Query().Get("after") == "" {
		return 0, nil
	}
	return strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
}

func Wait(r *http.Request) time.Duration {
	seconds, _ := strconv.Atoi(r.URL.Query().Get("wait"))
	if seconds < 0 {
		seconds = 0
	}
	if seconds > 20 {
		seconds = 20
	}
	return time.Duration(seconds) * time.Second
}
