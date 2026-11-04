// Command wire-proxy records the calls an app makes to its vendors.
//
//	wire-proxy --proxy stripe=127.0.0.1:5301 --proxy plaid=127.0.0.1:5302 --out calls.json
//
// Each --proxy listens on an address and forwards to that vendor's app
// emulator: the one at <VENDOR>_EMULATOR_URL if that is set, otherwise the one
// LocalStack serves. Point the app's vendor base URL at the listening address.
//
// Every call is recorded, with credentials in the bodies masked, in one
// recording shared by all the proxies. --events serves it over HTTP while the
// proxy runs, and --out writes it when the proxy stops.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

const usage = `usage: wire-proxy --proxy vendor=addr [--proxy vendor=addr ...] [--events addr] [--out file] [--keep n]

  --proxy   listen on addr and forward to the vendor's app emulator (repeatable)
  --events  serve the recording at http://addr/events?since=N
  --out     write the recording to file when the proxy stops
  --keep    calls to keep in memory (default 20000)
`

type proxySpec struct{ vendor, addr string }

type proxyFlags []proxySpec

func (p *proxyFlags) String() string { return "" }

func (p *proxyFlags) Set(v string) error {
	vendor, addr, ok := strings.Cut(v, "=")
	if !ok || vendor == "" || addr == "" {
		return fmt.Errorf("--proxy %q must be vendor=addr, such as stripe=127.0.0.1:5301", v)
	}
	for _, s := range *p {
		if s.vendor == vendor {
			return fmt.Errorf("--proxy names %s twice", vendor)
		}
	}
	*p = append(*p, proxySpec{vendor, addr})
	return nil
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	var proxies proxyFlags
	cfg := config{getenv: getenv, keep: 20000}
	fs := newFlagSet(stderr)
	fs.Var(&proxies, "proxy", "")
	fs.StringVar(&cfg.events, "events", "", "")
	fs.StringVar(&cfg.out, "out", "", "")
	fs.IntVar(&cfg.keep, "keep", cfg.keep, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if len(proxies) == 0 || fs.NArg() > 0 || cfg.keep < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cfg.proxies = proxies

	inst, err := start(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "wire-proxy: %v\n", err)
		return 1
	}
	for _, s := range cfg.proxies {
		t := inst.targets[s.vendor]
		fmt.Fprintf(stdout, "%-7s http://%s -> %s %s\n", s.vendor, inst.addrs[s.vendor], t.Source, t.Upstream)
	}
	if inst.eventsAddr != "" {
		fmt.Fprintf(stdout, "events  http://%s/events\n", inst.eventsAddr)
	}

	select {
	case <-ctx.Done():
	case err := <-inst.failed:
		fmt.Fprintf(stderr, "wire-proxy: %v\n", err)
		inst.stop(context.Background())
		return 1
	}
	if err := inst.stop(context.Background()); err != nil {
		fmt.Fprintf(stderr, "wire-proxy: %v\n", err)
		return 1
	}
	if cfg.out != "" {
		fmt.Fprintf(stdout, "wrote %d calls to %s\n", len(inst.rec.Events()), cfg.out)
	}
	return 0
}

type config struct {
	proxies []proxySpec
	events  string
	out     string
	keep    int
	getenv  func(string) string
}

// instance is a running wire-proxy.
type instance struct {
	rec        *wire.Recorder
	targets    map[string]wire.Target
	addrs      map[string]string // vendor to the address it listens on
	eventsAddr string
	out        string
	servers    []*http.Server
	failed     chan error
}

func start(cfg config) (*instance, error) {
	inst := &instance{
		rec:     wire.NewRecorder(cfg.keep),
		targets: map[string]wire.Target{},
		addrs:   map[string]string{},
		out:     cfg.out,
		failed:  make(chan error, len(cfg.proxies)+1),
	}
	serve := func(addr string, h http.Handler) (string, error) {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return "", err
		}
		srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
		inst.servers = append(inst.servers, srv)
		go func() {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				inst.failed <- err
			}
		}()
		return ln.Addr().String(), nil
	}

	for _, s := range cfg.proxies {
		t := wire.TargetFromEnv(cfg.getenv, s.vendor)
		addr, err := serve(s.addr, wire.NewProxy(t, inst.rec))
		if err != nil {
			inst.shutdown(context.Background())
			return nil, fmt.Errorf("listen for %s on %s: %w", s.vendor, s.addr, err)
		}
		inst.targets[s.vendor], inst.addrs[s.vendor] = t, addr
	}
	if cfg.events != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /events", inst.serveEvents)
		addr, err := serve(cfg.events, mux)
		if err != nil {
			inst.shutdown(context.Background())
			return nil, fmt.Errorf("listen for events on %s: %w", cfg.events, err)
		}
		inst.eventsAddr = addr
	}
	return inst, nil
}

// recording is what --events serves and --out writes. It is the shape of the
// demo's GET /api/events, so one reader handles both.
type recording struct {
	Events     []wire.Event `json:"events"`
	Last       uint64       `json:"last"`
	Generation uint64       `json:"generation"`
}

func (i *instance) snapshot(since uint64) recording {
	evs := i.rec.Since(since)
	last := since
	if len(evs) > 0 {
		last = evs[len(evs)-1].ID
	}
	return recording{Events: evs, Last: last, Generation: i.rec.Generation()}
}

func (i *instance) serveEvents(w http.ResponseWriter, r *http.Request) {
	var since uint64
	if s := r.URL.Query().Get("since"); s != "" {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			http.Error(w, "since must be an event id", http.StatusBadRequest)
			return
		}
		since = n
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(i.snapshot(since))
}

// shutdown closes every listener and waits for calls in flight.
func (i *instance) shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, s := range i.servers {
		_ = s.Shutdown(ctx)
	}
}

// stop closes every listener, then writes the recording to --out if it was
// asked for. The file is written last so it holds every call that finished.
func (i *instance) stop(ctx context.Context) error {
	i.shutdown(ctx)
	if i.out == "" {
		return nil
	}
	return writeAtomic(i.out, i.snapshot(0))
}

// writeAtomic writes v as JSON to path through a temporary file, so a reader
// never sees half a recording.
func writeAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wire-proxy-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
