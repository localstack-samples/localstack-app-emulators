// Package emulator starts app emulator binaries as child processes and waits
// until they are healthy. It is how the offline kit runs the app emulators
// without LocalStack.
//
// The health contract is the one LocalStack's apps plugin uses: an app emulator
// started with --port=N is ready when GET /admin/health answers exactly
// {"status":"ok"}.
package emulator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Spec describes one app emulator process.
type Spec struct {
	Name   string    // vendor name, used in errors
	Binary string    // path to the app emulator binary
	Port   int       // port passed as --port
	Args   []string  // extra arguments after --port
	Env    []string  // extra KEY=VALUE entries added to the inherited environment
	Output io.Writer // child stdout and stderr; discarded when nil
}

// URL is the base URL the app emulator serves on.
func (s Spec) URL() string { return fmt.Sprintf("http://127.0.0.1:%d", s.Port) }

// Process is a running app emulator.
type Process struct {
	Spec Spec
	cmd  *exec.Cmd
	done chan struct{} // closed when the process has exited
}

// Start launches the binary and returns once it reports healthy. It fails if
// ctx ends first or the process exits before becoming healthy.
func Start(ctx context.Context, s Spec) (*Process, error) {
	args := append([]string{fmt.Sprintf("--port=%d", s.Port)}, s.Args...)
	cmd := exec.Command(s.Binary, args...) // #nosec G204 -- the binary path is chosen by the demo, not by request input
	cmd.Env = append(os.Environ(), s.Env...)
	cmd.Stdout, cmd.Stderr = s.Output, s.Output
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", s.Name, err)
	}
	p := &Process{Spec: s, cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(p.done)
	}()

	// Stop waiting if the process dies, so a bad binary fails fast.
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-p.done:
			cancel()
		case <-waitCtx.Done():
		}
	}()

	if err := WaitHealthy(waitCtx, s.URL(), 100*time.Millisecond); err != nil {
		p.Stop()
		select {
		case <-p.done:
			if ctx.Err() == nil {
				return nil, fmt.Errorf("%s exited before becoming healthy", s.Name)
			}
		default:
		}
		return nil, fmt.Errorf("%s did not become healthy: %w", s.Name, err)
	}
	return p, nil
}

// Stop asks the process to exit and kills it if it has not within five seconds.
func (p *Process) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

// ErrNotHealthy is returned by WaitHealthy when ctx ends before the app
// emulator answers.
var ErrNotHealthy = errors.New("app emulator did not report healthy")

// WaitHealthy polls baseURL/admin/health until it answers exactly
// {"status":"ok"}, or ctx ends.
func WaitHealthy(ctx context.Context, baseURL string, interval time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		if healthy(ctx, client, baseURL) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ErrNotHealthy, ctx.Err())
		case <-time.After(interval):
		}
	}
}

func healthy(ctx context.Context, client *http.Client, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/admin/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	return len(body) == 1 && body["status"] == "ok"
}
