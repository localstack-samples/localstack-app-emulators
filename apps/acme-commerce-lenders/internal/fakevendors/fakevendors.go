// Package fakevendors is a stand-in for the Stripe, Plaid and Xero app
// emulators, for tests. One server answers for all three, because their routes
// do not overlap, so a test points every vendor at the same URL.
//
// It is as strict as the real app emulators about the one thing the seeders got
// wrong once: a Stripe /v1 call must carry a key.
package fakevendors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// New starts the server. resets counts POST /admin/reset calls.
func New(t *testing.T, resets *atomic.Int32) *httptest.Server {
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
		case p == "/item/get":
			w.Write([]byte(`{"item":{"institution_id":"ins_109508"}}`))
		case p == "/accounts/get":
			w.Write([]byte(`{"accounts":[{"account_id":"chk","subtype":"checking","mask":"0000","balances":{"current":110}}]}`))
		case p == "/transactions/get":
			w.Write([]byte(`{"total_transactions":2}`))
		case p == "/api.xro/2.0/Organisation":
			w.Write([]byte(`{"Organisations":[{"Name":"Demo Company (Global)"}]}`))
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
