package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"proposal-platform/api"
	"proposal-workflow/internal/service"
)

// Options selects the tools, project and resident processes for an explicit command.
type Options struct {
	Repository  string
	Main        string
	Project     string
	URL         string
	DecisionURL string
	Chow        string
	Renderer    string
}

func (o Options) request(ctx context.Context, method, path string, value, answer any) error {
	var body io.Reader
	if value != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(o.URL, "/")+path, body)
	if err != nil {
		return err
	}
	if value != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := (&http.Client{Timeout: 35 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			return err
		}
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, response.StatusCode, data)
	}
	return json.NewDecoder(response.Body).Decode(answer)
}

func commandID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func (o Options) submit(ctx context.Context, command service.Command) error {
	var err error
	if command.ID == "" {
		command.ID, err = commandID()
		if err != nil {
			return err
		}
	}
	var receipt api.Receipt
	if err = o.request(ctx, "POST", "/api/commands", command, &receipt); err != nil {
		return fmt.Errorf("command %s: %w", command.ID, err)
	}
	return printJSON(receipt)
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".proposal-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (o Options) frame(ctx context.Context, prefixes []string, epoch string, cursor uint64, wait bool) (api.Frame, error) {
	query := url.Values{}
	for _, prefix := range prefixes {
		query.Add("prefix", prefix)
	}
	query.Set("epoch", epoch)
	query.Set("after", fmt.Sprint(cursor))
	if wait {
		query.Set("wait", "20")
	}
	var frame api.Frame
	err := o.request(ctx, "GET", "/api/views?"+query.Encode(), nil, &frame)
	return frame, err
}

func (o Options) watch(ctx context.Context, uid string, attach bool) error {
	prefixes := []string{"event/", "decision/", "proposal/"}
	if uid != "" {
		prefixes = []string{"event/" + uid + "/", "decision/" + uid + "/", "proposal/" + uid}
	}
	epoch := ""
	var cursor uint64
	for {
		frame, err := o.frame(ctx, prefixes, epoch, cursor, true)
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(frame.Puts))
		for key := range frame.Puts {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			raw := frame.Puts[key]
			if attach && key == "proposal/"+uid {
				var status struct {
					Status string `json:"status"`
					Reason string `json:"reason"`
				}
				if err := json.Unmarshal(raw, &status); err != nil {
					return err
				}
				if status.Status != "running" {
					if err := printJSON(json.RawMessage(raw)); err != nil {
						return err
					}
					if status.Status == "present" {
						return nil
					}
					return fmt.Errorf("%s: %s: %s", uid, status.Status, status.Reason)
				}
			}
			if strings.HasPrefix(key, "event/") || strings.HasPrefix(key, "decision/") {
				if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"key": key, "value": json.RawMessage(raw)}); err != nil {
					return err
				}
			}
		}
		epoch, cursor = frame.Epoch, frame.Cursor
	}
}
