//go:build integration

// Package integration runs the demo against LocalStack and drives it over HTTP.
// It starts the demo in the test process, so it needs only LocalStack running
// with the Stripe, Plaid and Xero app emulators:
//
//	lstk
//	INTEGRATION_ALLOW_RESET=1 go test -tags integration ./integration
//
// The tests reset every app emulator they talk to: LocalStack's shared Stripe
// emulator and any emulator at <VENDOR>_EMULATOR_URL. They refuse to run
// unless INTEGRATION_ALLOW_RESET=1 is set.
//
// The tests share one seeded world, so they run in order and the last one
// resets it.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/internal/cli"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/lender"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
)

var (
	client = &http.Client{Timeout: 3 * time.Minute}
	base   string
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	if os.Getenv("INTEGRATION_ALLOW_RESET") != "1" {
		fmt.Fprintln(os.Stderr, "this resets every app emulator it talks to; set INTEGRATION_ALLOW_RESET=1 to run")
		return 1
	}
	dir, err := os.MkdirTemp("", "demo-integration")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	// A free port, so the tests never collide with a demo you have running.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	addr := l.Addr().String()
	l.Close()
	base = "http://" + addr

	go cli.Run(context.Background(), []string{"serve", "--addr", addr, "--state", filepath.Join(dir, "state.json")}, os.Getenv, os.Stderr, os.Stderr)
	for i := 0; ; i++ {
		if resp, err := client.Get(base + "/api/status"); err == nil {
			resp.Body.Close()
			break
		} else if i == 50 {
			fmt.Fprintln(os.Stderr, "the demo did not start:", err)
			return 1
		}
		time.Sleep(100 * time.Millisecond)
	}
	return m.Run()
}

// call sends a request to the demo and decodes a JSON response into out, which
// may be nil. It returns the HTTP status.
func call(t *testing.T, method, path string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, base+path, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			t.Fatalf("%s %s: status %d, body is not JSON: %s", method, path, resp.StatusCode, body)
		}
	}
	return resp.StatusCode
}

type status struct {
	Ready     bool `json:"ready"`
	Seeded    bool `json:"seeded"`
	Emulators []struct {
		Vendor string `json:"vendor"`
		State  string `json:"state"`
	} `json:"emulators"`
	Facts *world.Facts `json:"facts"`
}

// resetWorld puts every app emulator back at the seeded start.
func resetWorld(t *testing.T) world.Facts {
	t.Helper()
	var f struct {
		world.Facts
		Error string `json:"error"`
	}
	if code := call(t, "POST", "/api/reset", &f); code != http.StatusOK {
		t.Fatalf("reset: status %d: %s", code, f.Error)
	}
	return f.Facts
}

func TestSeededWorldIsReadyAndMatchesTheDocs(t *testing.T) {
	resetWorld(t)

	var st status
	if code := call(t, "GET", "/api/status", &st); code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if !st.Ready || !st.Seeded || st.Facts == nil {
		t.Fatalf("want a ready, seeded world, got %+v", st)
	}
	if len(st.Emulators) != 3 {
		t.Fatalf("want stripe, plaid and xero, got %+v", st.Emulators)
	}

	// The numbers in docs/usage.md. If an app emulator changes behavior, they move.
	f := st.Facts
	if f.Stripe.Orders != 941 || f.Stripe.GrossCents != 8108384 || f.Stripe.Payouts != 12 {
		t.Errorf("stripe facts: %+v", f.Stripe)
	}
	if f.Xero.Invoices != 6 || f.Xero.OutstandingCents != 1615000 || f.Xero.OverdueCents != 540000 {
		t.Errorf("xero facts: %+v", f.Xero)
	}
	if f.Plaid.AccountID == "" {
		t.Errorf("plaid facts: %+v", f.Plaid)
	}
}

func TestApplyAcceptFundAndSettle(t *testing.T) {
	resetWorld(t)

	var a lender.Application
	if code := call(t, "POST", "/api/applications", &a); code != http.StatusCreated || a.Status != lender.Applied {
		t.Fatalf("apply: %d %+v", code, a)
	}

	for _, step := range []struct {
		path string
		want lender.Status
	}{
		{"connect", lender.Connected},
		{"review", lender.Reviewed},
		{"offer", lender.Offered},
		{"accept", lender.Accepted},
		{"fund", lender.Funded},
		{"settle", lender.Settled},
	} {
		if code := call(t, "POST", "/api/applications/"+a.ID+"/"+step.path, &a); code != http.StatusOK {
			t.Fatalf("%s: status %d", step.path, code)
		}
		if a.Status != step.want {
			t.Fatalf("after %s the application is %s, want %s", step.path, a.Status, step.want)
		}
	}

	if len(a.Connections) != 3 {
		t.Errorf("want a connection per vendor, got %+v", a.Connections)
	}
	if a.Offer == nil || !a.Offer.Approved || a.Offer.AdvanceCents <= 0 {
		t.Fatalf("want an approved offer, got %+v", a.Offer)
	}
	if a.Offer.TotalRepayCents != a.Offer.AdvanceCents+a.Offer.FeeCents {
		t.Errorf("repay %d is not advance %d plus fee %d", a.Offer.TotalRepayCents, a.Offer.AdvanceCents, a.Offer.FeeCents)
	}

	// The money moved through Stripe, and the same amount is booked in Xero.
	fu := a.Funding
	if fu == nil || fu.TransferID == "" || fu.PayoutID == "" || fu.AmountCents != a.Offer.AdvanceCents {
		t.Fatalf("funding: %+v", fu)
	}
	se := a.Settlement
	if se == nil || se.AmountCents != a.Offer.AdvanceCents || se.Reference != a.ID || se.BankTransactionID == "" {
		t.Fatalf("settlement: %+v", se)
	}

	// What the server keeps is what the last step returned.
	var got lender.Application
	if code := call(t, "GET", "/api/applications/"+a.ID, &got); code != http.StatusOK || got.Status != lender.Settled {
		t.Fatalf("get: %d %+v", code, got)
	}
}

func TestStepsOutOfOrderAreRefused(t *testing.T) {
	resetWorld(t)

	var a lender.Application
	call(t, "POST", "/api/applications", &a)
	if code := call(t, "POST", "/api/applications/"+a.ID+"/fund", nil); code != http.StatusConflict {
		t.Errorf("fund before accept: status %d, want 409", code)
	}
	if code := call(t, "GET", "/api/applications/app-9999", nil); code != http.StatusNotFound {
		t.Errorf("unknown application: status %d, want 404", code)
	}
}

func TestEveryVendorCallIsRecordedAsAnEmulatorAnswer(t *testing.T) {
	resetWorld(t)

	var a lender.Application
	call(t, "POST", "/api/applications", &a)
	call(t, "POST", "/api/applications/"+a.ID+"/connect", &a)
	call(t, "POST", "/api/applications/"+a.ID+"/review", &a)

	var ev struct {
		Events []struct {
			Vendor string `json:"vendor"`
			Origin string `json:"origin"`
			Status int    `json:"status"`
			Error  string `json:"error"`
		} `json:"events"`
	}
	if code := call(t, "GET", "/api/events", &ev); code != http.StatusOK {
		t.Fatalf("events: %d", code)
	}
	seen := map[string]bool{}
	for _, e := range ev.Events {
		seen[e.Vendor] = true
		if e.Origin != "emulator" {
			t.Errorf("%s call has origin %q, want emulator", e.Vendor, e.Origin)
		}
		if e.Error != "" || e.Status >= 500 {
			t.Errorf("%s call failed: %d %s", e.Vendor, e.Status, e.Error)
		}
	}
	for _, v := range []string{"stripe", "plaid", "xero"} {
		if !seen[v] {
			t.Errorf("no recorded call to %s", v)
		}
	}
}

// Last, because it leaves the world as it found it: a reset undoes a run.
func TestResetUndoesARun(t *testing.T) {
	before := resetWorld(t)

	var a lender.Application
	call(t, "POST", "/api/applications", &a)
	call(t, "POST", "/api/applications/"+a.ID+"/connect", &a)

	after := resetWorld(t)
	if after.Stripe.Orders != before.Stripe.Orders || after.Stripe.GrossCents != before.Stripe.GrossCents {
		t.Errorf("the world changed across a reset: %+v then %+v", before.Stripe, after.Stripe)
	}
	var st status
	call(t, "GET", "/api/status", &st)
	if !st.Ready {
		t.Errorf("not ready after a reset: %+v", st)
	}
}
