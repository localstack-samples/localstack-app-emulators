package world

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// verifyRig is three app emulators that each hold the seeded world, and the
// facts that describe it. A test removes what one of them holds.
func verifyRig(t *testing.T) (Targets, Facts, map[string]*fake) {
	t.Helper()
	fakes := map[string]*fake{
		"stripe": newFake(t, map[string]string{"GET /v1/accounts/acct_1": `{"id":"acct_1"}`}),
		"plaid":  newFake(t, map[string]string{"POST /item/get": `{"item":{}}`}),
		"xero":   newFake(t, map[string]string{"GET /api.xro/2.0/Invoices": `{"Invoices":[{"InvoiceID":"a"},{"InvoiceID":"b"}]}`}),
	}
	tg := Targets{
		Stripe: wire.Direct("stripe", fakes["stripe"].srv.URL),
		Plaid:  wire.Direct("plaid", fakes["plaid"].srv.URL),
		Xero:   wire.Direct("xero", fakes["xero"].srv.URL),
	}
	f := Facts{
		Stripe: StripeFacts{AccountID: "acct_1"},
		Plaid:  PlaidFacts{AccessToken: "access-sandbox-1"},
		Xero:   XeroFacts{TenantID: XeroTenantID, Invoices: 2},
	}
	return tg, f, fakes
}

func TestVerifyFindsNothingWhenEveryAppEmulatorHoldsTheWorld(t *testing.T) {
	tg, f, _ := verifyRig(t)
	if got := Verify(context.Background(), nil, tg, f); len(got) != 0 {
		t.Fatalf("want a world that checks out, got %+v", got)
	}
}

func TestVerifyReportsAPlaidTokenTheAppEmulatorNoLongerKnows(t *testing.T) {
	// The case that showed "Ready" over a broken world: Plaid restarted, so the
	// access token saved at seeding is unknown to it.
	tg, f, fakes := verifyRig(t)
	delete(fakes["plaid"].routes, "POST /item/get")

	got := Verify(context.Background(), nil, tg, f)
	if len(got) != 1 || got[0].Vendor != "plaid" {
		t.Fatalf("want plaid reported, got %+v", got)
	}
	if calls := fakes["plaid"].to("POST", "/item/get"); len(calls) != 1 || !strings.Contains(calls[0].Body, "access-sandbox-1") {
		t.Fatalf("want one /item/get call carrying the saved token, got %+v", calls)
	}
}

func TestVerifyReportsAMissingStripeAccountAndShortXeroInvoices(t *testing.T) {
	tg, f, fakes := verifyRig(t)
	delete(fakes["stripe"].routes, "GET /v1/accounts/acct_1")
	fakes["xero"].routes["GET /api.xro/2.0/Invoices"] = `{"Invoices":[{"InvoiceID":"a"}]}`

	got := Verify(context.Background(), nil, tg, f)
	vendors := []string{}
	for _, s := range got {
		vendors = append(vendors, s.Vendor)
	}
	if strings.Join(vendors, ",") != "stripe,xero" {
		t.Fatalf("want stripe and xero reported, got %+v", got)
	}
}

func TestVerifyDoesNotCallASlowOrDownAppEmulatorStale(t *testing.T) {
	// A server error or a refused connection says nothing about what the app
	// emulator holds, and the readiness check already reports it.
	tg, f, fakes := verifyRig(t)
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer broken.Close()
	tg.Stripe = wire.Direct("stripe", broken.URL)
	fakes["plaid"].srv.Close()

	if got := Verify(context.Background(), nil, tg, f); len(got) != 0 {
		t.Fatalf("want nothing reported for a 500 or a refused connection, got %+v", got)
	}
}
