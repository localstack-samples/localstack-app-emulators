package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/internal/api"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/internal/ui"
	"github.com/localstack-samples/localstack-app-emulators/shared/vendorcfg"
)

// doctor runs every check, prints one line each with the fix for a failure, and
// fails if any check failed. It changes nothing.
func (a *app) doctor(addr string) error {
	bad := 0
	report := func(ok bool, what, fix string) {
		if ok {
			fmt.Fprintf(a.out, "  ok      %s\n", what)
			return
		}
		bad++
		fmt.Fprintf(a.out, "  FAILED  %s\n          %s\n", what, fix)
	}

	fmt.Fprintln(a.out, "This machine:")
	report(ui.Built(), "the web UI is built into this binary",
		"this binary has only the placeholder page; run: make web APP=acme-commerce-lenders, then rebuild")
	report(a.stateWritable(), "the state file can be written at "+a.statePath,
		"make that folder writable, or choose another with --state")
	if err := portFree(addr); err != nil {
		report(false, "the demo can listen on "+addr,
			"something is already on that port, often an earlier run; stop it, or use serve --addr 127.0.0.1:8081")
	} else {
		report(true, "the demo can listen on "+addr, "")
	}

	fmt.Fprintln(a.out, "App emulators:")
	store, err := a.store()
	if err != nil {
		return err
	}
	st, err := api.WorldAdmin{Targets: a.targets, HC: a.httpClient(), Store: store}.Status(a.ctx)
	if err != nil {
		report(false, "the state file can be read", err.Error()+"; delete it and run: reset")
		return fmt.Errorf("%d problem(s) found", bad)
	}
	for _, e := range st.Emulators {
		report(e.OK(), fmt.Sprintf("%s (%s) %s at %s", e.Vendor, e.Source, e.State, e.URL), e.Hint)
	}
	for _, t := range a.targets.All() {
		if t.Source == "localstack" {
			continue
		}
		if a.getenv(vendorcfg.EmulatorVersionEnvName(t.Vendor)) == "" {
			fmt.Fprintf(a.out, "  note    %s has no version recorded; set %s to show it in the inspector\n", t.Vendor, vendorcfg.EmulatorVersionEnvName(t.Vendor))
		}
	}

	fmt.Fprintln(a.out, "The world:")
	switch {
	case !st.Seeded:
		report(false, "the world is seeded", "run: reset (or press Reset in the UI)")
	case len(st.Stale) > 0:
		for _, s := range st.Stale {
			report(false, fmt.Sprintf("%s still holds the seeded world", s.Vendor), s.Reason+"; run: reset")
		}
	default:
		report(true, "the world is seeded and every app emulator still holds it", "")
	}

	if bad > 0 {
		return fmt.Errorf("%d problem(s) found", bad)
	}
	fmt.Fprintln(a.out, "all checks passed")
	return nil
}

// stateWritable reports whether the state file's folder exists or can be made,
// and can be written to. It leaves nothing behind.
func (a *app) stateWritable() bool {
	dir := filepath.Dir(a.statePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name) == nil
}

func portFree(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return l.Close()
}
