package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/internal/fakevendors"
)

func run(t *testing.T, env map[string]string, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	args = append(args, "--state", filepath.Join(t.TempDir(), "state.json"))
	code = Run(context.Background(), args, func(k string) string { return env[k] }, &o, &e)
	return code, o.String(), e.String()
}

func envFor(url string) map[string]string {
	return map[string]string{"STRIPE_EMULATOR_URL": url, "PLAID_EMULATOR_URL": url, "XERO_EMULATOR_URL": url}
}

func TestUnknownCommandIsRefused(t *testing.T) {
	code, _, errOut := run(t, nil, "explode")
	if code != 2 || !strings.Contains(errOut, `unknown command "explode"`) || !strings.Contains(errOut, "Usage:") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestHelpPrintsUsageAndSucceeds(t *testing.T) {
	code, out, _ := run(t, nil, "help")
	if code != 0 || !strings.Contains(out, "<VENDOR>_EMULATOR_URL") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestReadySaysWhatToDoAboutAnAppEmulatorThatIsDown(t *testing.T) {
	var resets atomic.Int32
	up := fakevendors.New(t, &resets)
	down := httptest.NewServer(http.NotFoundHandler())
	downURL := down.URL
	down.Close()
	env := envFor(up.URL)
	env["XERO_EMULATOR_URL"] = downURL

	code, out, errOut := run(t, env, "ready")
	if code != 1 || !strings.Contains(out, "unreachable") || !strings.Contains(out, "XERO_EMULATOR_URL") || !strings.Contains(errOut, "1 of 3") {
		t.Fatalf("code=%d\nout=%s\nerr=%s", code, out, errOut)
	}
}

func TestReadySucceedsWhenEverythingIsUp(t *testing.T) {
	var resets atomic.Int32
	code, out, _ := run(t, envFor(fakevendors.New(t, &resets).URL), "ready")
	if code != 0 || !strings.Contains(out, "all app emulators are ready") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestSeedThenSeedAgainAsksForAReset(t *testing.T) {
	var resets atomic.Int32
	env := envFor(fakevendors.New(t, &resets).URL)
	state := filepath.Join(t.TempDir(), "state.json")

	var o, e bytes.Buffer
	getenv := func(k string) string { return env[k] }
	if code := Run(context.Background(), []string{"seed", "--state", state}, getenv, &o, &e); code != 0 {
		t.Fatalf("first seed failed: %s", e.String())
	}
	for _, want := range []string{"Seeding:", "Harbor & Pine Outfitters", "941 orders", "$81,083.84 gross", "12 payouts", "$16,150.00 outstanding", "$5,400.00 overdue", "2 transactions"} {
		if !strings.Contains(o.String(), want) {
			t.Errorf("the summary should say %q:\n%s", want, o.String())
		}
	}

	o.Reset()
	e.Reset()
	if code := Run(context.Background(), []string{"seed", "--state", state}, getenv, &o, &e); code != 1 || !strings.Contains(e.String(), "run: reset") {
		t.Fatalf("a second seed should tell the presenter to reset: code=%d err=%s", code, e.String())
	}

	// Reset on the same state works and wipes all three.
	o.Reset()
	e.Reset()
	if code := Run(context.Background(), []string{"reset", "--state", state}, getenv, &o, &e); code != 0 {
		t.Fatalf("reset failed: %s", e.String())
	}
	if resets.Load() != 3 || !strings.Contains(o.String(), "Reset wipes these app emulators") {
		t.Fatalf("reset should say what it wipes and wipe all three: resets=%d\n%s", resets.Load(), o.String())
	}
}

func TestResetNamesTheTargetsBeforeWipingThem(t *testing.T) {
	var resets atomic.Int32
	srv := fakevendors.New(t, &resets)
	_, out, _ := run(t, envFor(srv.URL), "reset")
	for _, v := range []string{"stripe", "plaid", "xero"} {
		if !strings.Contains(out, v+" ") || !strings.Contains(out, srv.URL) {
			t.Errorf("reset should list %s and its URL before wiping:\n%s", v, out)
		}
	}
	if strings.Index(out, "Reset wipes") > strings.Index(out, "ok") {
		t.Errorf("targets must be printed before the first step runs:\n%s", out)
	}
}

func TestDollars(t *testing.T) {
	for cents, want := range map[int64]string{0: "$0.00", 5: "$0.05", 11000: "$110.00", 540000: "$5,400.00", 8108384: "$81,083.84", 123456789: "$1,234,567.89"} {
		if got := dollars(cents); got != want {
			t.Errorf("dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}
