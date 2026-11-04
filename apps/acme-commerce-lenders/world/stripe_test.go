package world

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// fakeStripe records what a seeder does to it.
type fakeStripe struct {
	mu       sync.Mutex
	srv      *httptest.Server
	accounts int
	external int
	intents  []url.Values
	intentAc []string // the Stripe-Account header on each intent
	funded   []int64
	payouts  []int64
	advances int
	resets   int
	log      []string // "fund" and "payout" in the order they happened
}

func newFakeStripe(t *testing.T) *fakeStripe {
	f := &fakeStripe{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/v1/") && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk_") {
			// The real app emulator answers 401 to a /v1 call with no key.
			http.Error(w, `{"error":{"message":"You did not provide an API key."}}`, http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/accounts":
			f.accounts++
			w.Write([]byte(`{"id":"acct_merchant"}`))
		case r.URL.Path == "/v1/accounts/acct_merchant/external_accounts":
			f.external++
			w.Write([]byte(`{"id":"ba_1"}`))
		case r.URL.Path == "/v1/payment_intents":
			f.intents = append(f.intents, form)
			f.intentAc = append(f.intentAc, r.Header.Get("Stripe-Account"))
			w.Write([]byte(`{"id":"pi_x","status":"succeeded"}`))
		case r.URL.Path == "/admin/accounts/acct_merchant/fund":
			var b struct{ Amount int64 }
			json.Unmarshal(raw, &b)
			f.funded = append(f.funded, b.Amount)
			f.log = append(f.log, "fund")
			w.Write([]byte(`{}`))
		case r.URL.Path == "/v1/payouts":
			n, _ := strconv.ParseInt(form.Get("amount"), 10, 64)
			f.payouts = append(f.payouts, n)
			f.log = append(f.log, "payout")
			w.Write([]byte(`{"id":"po_x","status":"pending"}`))
		case r.URL.Path == "/admin/time/advance":
			f.advances++
			w.Write([]byte(`{}`))
		case r.URL.Path == "/admin/health":
			w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/admin/reset":
			f.resets++
			w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestSeedStripeBuildsTheMerchantsHistory(t *testing.T) {
	f := newFakeStripe(t)
	facts, err := SeedStripe(context.Background(), nil, wire.Direct("stripe", f.srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	orders := Orders()

	if f.accounts != 1 || f.external != 1 || facts.AccountID != "acct_merchant" {
		t.Fatalf("one connected account with one bank account: accounts=%d external=%d id=%q", f.accounts, f.external, facts.AccountID)
	}
	if len(f.intents) != len(orders) || facts.Orders != len(orders) || facts.GrossCents != Gross(orders) {
		t.Fatalf("every order is a payment intent: intents=%d orders=%d gross=%d want %d", len(f.intents), facts.Orders, facts.GrossCents, Gross(orders))
	}
	for i, ac := range f.intentAc {
		if ac != "acct_merchant" {
			t.Fatalf("payment intent %d was not made on the merchant's account (Stripe-Account %q)", i, ac)
		}
	}
	for i, in := range f.intents {
		if in.Get("amount") != strconv.FormatInt(orders[i].Cents, 10) || in.Get("confirm") != "true" {
			t.Fatalf("payment intent %d does not match order %d: %v", i, i, in)
		}
	}
	if f.advances != Days {
		t.Fatalf("the clock moves one day per day of history: %d advances for %d days", f.advances, Days)
	}
}

func TestSeedStripePaysOutEachFullWeekAfterFundingExactlyThatAmount(t *testing.T) {
	f := newFakeStripe(t)
	facts, err := SeedStripe(context.Background(), nil, wire.Direct("stripe", f.srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	weeks := Days / 7
	if facts.Payouts != weeks || len(f.payouts) != weeks || len(f.funded) != weeks {
		t.Fatalf("want %d weekly payouts, got facts=%d payouts=%d funded=%d", weeks, facts.Payouts, len(f.payouts), len(f.funded))
	}
	var paid int64
	for i := range f.payouts {
		if f.payouts[i] != f.funded[i] {
			t.Fatalf("week %d: funded %d but paid out %d, which would leave a balance", i, f.funded[i], f.payouts[i])
		}
		paid += f.payouts[i]
	}
	if paid != facts.PayoutCents {
		t.Fatalf("payout total %d does not match the facts %d", paid, facts.PayoutCents)
	}
	// Each payout is funded first, then paid.
	for i := 0; i+1 < len(f.log); i += 2 {
		if f.log[i] != "fund" || f.log[i+1] != "payout" {
			t.Fatalf("fund must come right before its payout: %v", f.log[i:i+2])
		}
	}
	if paid >= facts.GrossCents {
		t.Fatalf("the last partial week has no payout yet, so payouts %d must be less than gross %d", paid, facts.GrossCents)
	}
}

func TestSeedStripeNamesWhereItFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/accounts":
			w.Write([]byte(`{"id":"acct_merchant"}`))
		case strings.HasSuffix(r.URL.Path, "/external_accounts"):
			w.Write([]byte(`{}`))
		default:
			http.Error(w, `{"error":"declined"}`, http.StatusPaymentRequired)
		}
	}))
	defer srv.Close()
	_, err := SeedStripe(context.Background(), nil, wire.Direct("stripe", srv.URL))
	if err == nil || !strings.Contains(err.Error(), "day 0, order 0") || !strings.Contains(err.Error(), "402") {
		t.Fatalf("the error should say which order failed and why: %v", err)
	}
}

func TestSeedStripeSendsTheLendersKeyOnEveryCall(t *testing.T) {
	var unauthenticated []string
	f := newFakeStripe(t)
	inner := f.srv.Config.Handler
	f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") && r.Header.Get("Authorization") != "Bearer "+stripeKey {
			unauthenticated = append(unauthenticated, r.URL.Path)
		}
		inner.ServeHTTP(w, r)
	})
	if _, err := SeedStripe(context.Background(), nil, wire.Direct("stripe", f.srv.URL)); err != nil {
		t.Fatal(err)
	}
	if len(unauthenticated) != 0 {
		t.Fatalf("calls without the lender's key: %v", unauthenticated[:min(3, len(unauthenticated))])
	}
}
