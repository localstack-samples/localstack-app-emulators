// Package cli is the demo's command line: serve the UI, check that the app
// emulators are ready, seed the world, and reset it.
package cli

import (
	"context"
	"errors"
	"flag"
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

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/internal/api"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/internal/ui"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/lender"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

const usage = `Usage: acme-commerce-lenders [command] [flags]

Commands:
  serve   serve the demo UI (the default)
  ready   check that the Stripe, Plaid and Xero app emulators are up
  seed    seed the world into the app emulators
  reset   wipe the app emulators and seed the world again

Each vendor is served by LocalStack unless <VENDOR>_EMULATOR_URL says you run
its app emulator yourself, for example PLAID_EMULATOR_URL=http://127.0.0.1:5101.
`

// Run runs the command in args and returns the process exit code.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	app := &app{ctx: ctx, getenv: getenv, out: stdout, err: stderr, targets: world.TargetsFromEnv(getenv)}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&app.statePath, "state", defaultStatePath(), "file holding what the lender knows")
	addr := fs.String("addr", defaultAddr, "address the UI listens on (serve only); when not given, the next free port is used if 8080 is taken")

	switch cmd {
	case "serve", "ready", "seed", "reset":
	case "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	explicit := false
	fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "addr" })

	var err error
	switch cmd {
	case "serve":
		err = app.serve(*addr, explicit)
	case "ready":
		err = app.ready()
	case "seed":
		err = app.seed()
	case "reset":
		err = app.reset()
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

type app struct {
	ctx       context.Context
	getenv    func(string) string
	out, err  io.Writer
	targets   world.Targets
	statePath string
}

// httpClient allows for LocalStack installing an app emulator on the first call.
func (a *app) httpClient() *http.Client { return &http.Client{Timeout: 90 * time.Second} }

func (a *app) store() (core.Store, error) { return core.NewFileStore(a.statePath) }

// handler is the whole demo over HTTP: the API under /api/ and the UI at /.
func (a *app) handler() (http.Handler, error) {
	store, err := a.store()
	if err != nil {
		return nil, err
	}
	rec := wire.NewRecorder(2000)
	sw := wire.NewSwitch(a.targets.Lanes()...)
	svc := &lender.Service{Vendors: lender.NewSwitchedVendors(rec, sw), Store: store}
	admin := api.WorldAdmin{Targets: a.targets, HC: a.httpClient(), Store: store, Rec: rec}
	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewWithTargets(svc, admin, rec, sw))
	mux.Handle("/", ui.Handler())
	return mux, nil
}

const defaultAddr = "127.0.0.1:8080"

// listen opens the UI's port. An address the presenter chose must be free or it
// fails. The default is the first free port from 8080 to 8089, so a stale run
// of the demo does not stop the next one.
func listen(addr string, explicit bool) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err == nil || explicit {
		return l, err
	}
	host, portStr, splitErr := net.SplitHostPort(addr)
	port, convErr := strconv.Atoi(portStr)
	if splitErr != nil || convErr != nil {
		return nil, err
	}
	for p := port + 1; p < port+10; p++ {
		if l, e := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p))); e == nil {
			return l, nil
		}
	}
	return nil, fmt.Errorf("%w (ports %d to %d are all taken; stop the earlier run, or choose one with --addr)", err, port, port+9)
}

func (a *app) serve(addr string, explicit bool) error {
	h, err := a.handler()
	if err != nil {
		return err
	}
	l, err := listen(addr, explicit)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	a.printTargets("Calling:")
	fmt.Fprintf(a.out, "ACME Commerce Lenders on http://%s\n", l.Addr())

	// Ctrl-C and a plain kill end the run cleanly, so the port is free at once.
	ctx, stop := signal.NotifyContext(a.ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func (a *app) printTargets(heading string) {
	fmt.Fprintln(a.out, heading)
	for _, t := range a.targets.All() {
		fmt.Fprintf(a.out, "  %-7s %-10s %s\n", t.Vendor, t.Source, t.Upstream)
	}
}

func (a *app) ready() error {
	statuses := seed.Ready(a.ctx, a.httpClient(), a.targets.All()...)
	bad := 0
	for _, s := range statuses {
		fmt.Fprintf(a.out, "  %-7s %-10s %-13s %s\n", s.Vendor, s.Source, s.State, s.URL)
		if !s.OK() {
			bad++
			fmt.Fprintf(a.out, "          %s\n", s.Hint)
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d app emulators are not ready", bad, len(statuses))
	}
	fmt.Fprintln(a.out, "all app emulators are ready")
	return nil
}

func (a *app) progress(e seed.StepEvent) {
	mark := "ok"
	if e.Err != nil {
		mark = "FAILED"
	}
	fmt.Fprintf(a.out, "  %-6s %s (%s)\n", mark, e.Step, e.Elapsed.Round(time.Millisecond))
}

func (a *app) seed() error {
	store, err := a.store()
	if err != nil {
		return err
	}
	a.printTargets("Seeding:")
	facts, err := world.Seed(a.ctx, a.httpClient(), a.targets, store, a.progress)
	if errors.Is(err, world.ErrAlreadySeeded) {
		return fmt.Errorf("%w (run: reset)", err)
	}
	if err != nil {
		return err
	}
	a.summary(facts)
	return nil
}

func (a *app) reset() error {
	store, err := a.store()
	if err != nil {
		return err
	}
	a.printTargets("Reset wipes these app emulators and seeds the world again:")
	facts, err := world.Reset(a.ctx, a.httpClient(), a.targets, store, a.progress)
	if err != nil {
		return err
	}
	a.summary(facts)
	return nil
}

func (a *app) summary(f world.Facts) {
	fmt.Fprintf(a.out, "\n%s, as of %s\n", f.Merchant, f.AsOf)
	fmt.Fprintf(a.out, "  stripe  %s: %d orders, %s gross, %d payouts\n", f.Stripe.AccountID, f.Stripe.Orders, dollars(f.Stripe.GrossCents), f.Stripe.Payouts)
	fmt.Fprintf(a.out, "  plaid   checking ending %s: %d transactions, balance %s (the app emulator's fixed fixture)\n", f.Plaid.Mask, f.Plaid.Transactions, dollars(f.Plaid.BalanceCents))
	fmt.Fprintf(a.out, "  xero    %d invoices: %s outstanding, %s overdue\n", f.Xero.Invoices, dollars(f.Xero.OutstandingCents), dollars(f.Xero.OverdueCents))
}

func dollars(cents int64) string {
	s := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	n := len(s) - 3
	for i := n - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "$" + s
}

func defaultStatePath() string {
	// The state is the lender's own record of the seeded world. Keep it in the
	// user's config folder, not a cache folder the system may clear: on a Mac,
	// ~/Library/Application Support rather than ~/Library/Caches.
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "localstack-app-emulators", "acme-commerce-lenders", "state.json")
}
