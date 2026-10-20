package highlight

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"proposal-platform/api"
	"strconv"
	"strings"
)

// readFiles uses one read-only Git batch for all complete old/new files.
// NUL framing preserves filenames with whitespace and newlines.
func readFiles(ctx context.Context, source api.Source, paths []string) (map[string]string, error) {
	specs := make([]string, 0, 2*len(paths))
	for _, path := range paths {
		specs = append(specs, source.Base+":"+path, source.Commit+":"+path)
	}
	return readBlobs(ctx, source.Repository, specs)
}

func readBlobs(ctx context.Context, repository string, specs []string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repository, "cat-file", "--batch", "-Z")
	cmd.Stdin = strings.NewReader(strings.Join(specs, "\x00") + "\x00")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	reader := bufio.NewReader(pipe)
	files := map[string]string{}
	total := 0
	for _, spec := range specs {
		header, err := reader.ReadString(0)
		if err != nil {
			return nil, err
		}
		header = strings.TrimSuffix(header, "\x00")
		if header == spec+" missing" {
			files[spec] = ""
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("unexpected Git object: %s", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid blob length")
		}
		total += size
		if total > 48<<20 {
			return nil, fmt.Errorf("highlight sources exceed 48 MiB")
		}
		data := make([]byte, size)
		if _, err = io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		terminator, err := reader.ReadByte()
		if err != nil || terminator != 0 {
			return nil, fmt.Errorf("invalid blob terminator")
		}
		files[spec] = string(data)
	}
	if err = cmd.Wait(); err != nil {
		return nil, err
	}
	return files, nil
}
