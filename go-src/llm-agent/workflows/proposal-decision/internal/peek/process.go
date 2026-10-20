package peek

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Process owns the shared @webui/peek server over a private Unix socket.
// Go owns its lifetime and public endpoint; chat JSON belongs to this program.
type Process struct {
	Handler   http.Handler
	command   *exec.Cmd
	done      chan error
	directory string
}

func Start(ctx context.Context, script, repository, state, contextURL string) (*Process, error) {
	directory, err := os.MkdirTemp("", "proposal-peek-")
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(directory, "http.sock")
	command := exec.CommandContext(ctx, "node", script, "--socket", socket, "--repository", repository, "--state", state, "--context-url", contextURL)
	command.Stderr = os.Stderr
	output, err := command.StdoutPipe()
	if err != nil {
		os.RemoveAll(directory)
		return nil, err
	}
	if err = command.Start(); err != nil {
		os.RemoveAll(directory)
		return nil, err
	}
	p := &Process{command: command, directory: directory, done: make(chan error, 1)}
	go func() { p.done <- command.Wait() }()
	ready := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(output)
		line, err := reader.ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected Peek startup: %q", line)
		}
		ready <- err
	}()
	select {
	case err = <-ready:
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(15 * time.Second):
		err = fmt.Errorf("Peek startup timed out")
	}
	if err != nil {
		p.Close()
		return nil, err
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	target := &url.URL{Scheme: "http", Host: "peek"}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = transport
	proxy.FlushInterval = -1
	p.Handler = proxy
	return p, nil
}

func (p *Process) Close() {
	_ = p.command.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		_ = p.command.Process.Kill()
		<-p.done
	}
	_ = os.RemoveAll(p.directory)
}
