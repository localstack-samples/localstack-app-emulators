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
)

// allVendors is one server that answers for Stripe, Plaid and Xero. Their
// routes do not overlap, so the CLI can be pointed at it three times.
func allVendors(t *testing.T, resets *atomic.Int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		if strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk_") {
			http.Error(w, `{"error":{"message":"You did not provide an API key."}}`, http.StatusUnauthorized)
			return
		}
		switch {
		case p == "/admin/health":
			w.Write([]byte(`{"status":"ok"}`))
		case p == "/admin/reset":
			resets.Add(1)
			w.Write([]byte(`{}`))
		case p == "/v1/accounts":
			w.Write([]byte(`{"id":"acct_merchant"}`))
		case strings.HasPrefix(p, "/v1/") || strings.HasPrefix(p, "/admin/"):
			w.Write([]byte(`{"id":"x","status":"succeeded"}`))
		case p == "/sandbox/public_token/create":
			w.Write([]byte(`{"public_token":"pub"}`))
		case p == "/item/public_token/exchange":
			w.Write([]byte(`{"access_token":"tok","item_id":"item"}`))
		case p == "/accounts/get":
			w.Write([]byte(`{"accounts":[{"account_id":"chk","subtype":"checking","mask":"0000","balances":{"current":110}}]}`))
		case p == "/transactions/get":
			w.Write([]byte(`{"total_transactions":2}`))
		case p == "/api.xro/2.0/Contacts":
			w.Write([]byte(`{"Contacts":[{"ContactID":"c0"},{"ContactID":"c1"},{"ContactID":"c2"}]}`))
		case p == "/api.xro/2.0/Invoices":
			w.Write([]byte(`{"Invoices":[{"InvoiceID":"i0"},{"InvoiceID":"i1"},{"InvoiceID":"i2"},{"InvoiceID":"i3"},{"InvoiceID":"i4"},{"InvoiceID":"i5"}]}`))
		case p == "/api.xro/2.0/Payments":
			w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

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
	up := allVendors(t, &resets)
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
	code, out, _ := run(t, envFor(allVendors(t, &resets).URL), "ready")
	if code != 0 || !strings.Contains(out, "all app emulators are ready") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestSeedThenSeedAgainAsksForAReset(t *testing.T) {
	var resets atomic.Int32
	env := envFor(allVendors(t, &resets).URL)
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
	srv := allVendors(t, &resets)
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
