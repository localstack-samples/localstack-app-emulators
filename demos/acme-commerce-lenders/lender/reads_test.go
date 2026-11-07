package lender

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// The world ends at a fixed moment, so every test sees the same windows.
var asOf = time.Date(2027, 2, 1, 12, 0, 0, 0, time.UTC)

func facts() world.Facts {
	return world.Facts{
		Merchant: world.MerchantName,
		AsOf:     asOf.Format("2006-01-02"),
		AsOfUnix: asOf.Unix(),
		Stripe:   world.StripeFacts{AccountID: "acct_m"},
		Plaid:    world.PlaidFacts{AccessToken: "access-secret-token", AccountID: "chk", Institution: "ins_109508"},
		Xero:     world.XeroFacts{TenantID: "tenant-1"},
	}
}

func asXeroDate(t time.Time) string { return fmt.Sprintf("/Date(%d+0000)/", t.UnixMilli()) }

// chargeListCalls counts GET /v1/charges requests, to prove the lender pages.
type fakeVendors struct {
	srv         *httptest.Server
	chargeCalls atomic.Int32
	noChecking  bool

	// What funding does to the fake, in the order it happened.
	mu          sync.Mutex
	order       []string // "external", "transfer", "payout"
	forms       map[string]url.Values
	headers     map[string]http.Header
	failPayouts int // how many payout requests to fail before one succeeds

	// The merchant's Xero books.
	xeroAccounts []string // account codes already in the chart
	xeroContacts []map[string]string
	bankTxs      []map[string]string
	bodies       map[string][]byte
	failBankTx   int // how many bank transaction requests to fail before one succeeds
}

// noteJSON records a JSON request and returns its body.
func (f *fakeVendors) noteJSON(kind string, r *http.Request) []byte {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bodies == nil {
		f.bodies = map[string][]byte{}
	}
	f.order = append(f.order, kind)
	f.bodies[kind] = body
	return body
}

func (f *fakeVendors) note(kind string, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.ParseForm()
	if f.forms == nil {
		f.forms, f.headers = map[string]url.Values{}, map[string]http.Header{}
	}
	f.order = append(f.order, kind)
	f.forms[kind], f.headers[kind] = r.PostForm, r.Header.Clone()
}

func (f *fakeVendors) count(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, k := range f.order {
		if k == kind {
			n++
		}
	}
	return n
}

func newFakeVendors(t *testing.T) *fakeVendors {
	f := &fakeVendors{}
	orders := world.Orders()
	// The seeded charges, newest first, created as the seeder created them:
	// day d is Days-d days before the history ends.
	type charge struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Created  int64  `json:"created"`
		Status   string `json:"status"`
		Refunded bool   `json:"refunded"`
	}
	var charges []charge
	for i := len(orders) - 1; i >= 0; i-- {
		o := orders[i]
		charges = append(charges, charge{
			ID: "ch_" + strconv.Itoa(i), Amount: o.Cents,
			Created: asOf.Unix() - int64(world.Days-o.Day)*86400 + int64(i%50),
			Status:  "succeeded",
		})
	}
	// One refunded and one failed charge must not count as sales.
	charges = append([]charge{
		{ID: "ch_refunded", Amount: 999999, Created: asOf.Unix() - 3600, Status: "succeeded", Refunded: true},
		{ID: "ch_failed", Amount: 888888, Created: asOf.Unix() - 3600, Status: "failed"},
	}, charges...)

	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		if strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk_") {
			http.Error(w, `{"error":"no key"}`, http.StatusUnauthorized)
			return
		}
		switch {
		case p == "/v1/accounts/acct_m":
			w.Write([]byte(`{"id":"acct_m"}`))
		case p == "/v1/charges":
			f.chargeCalls.Add(1)
			if r.Header.Get("Stripe-Account") != "acct_m" {
				http.Error(w, `{"error":"charges must be read on the merchant's account"}`, http.StatusBadRequest)
				return
			}
			start := 0
			if after := r.URL.Query().Get("starting_after"); after != "" {
				for i, c := range charges {
					if c.ID == after {
						start = i + 1
					}
				}
			}
			end := min(start+100, len(charges))
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": charges[start:end], "has_more": end < len(charges)})
		case p == "/v1/accounts/acct_m/external_accounts" && r.Method == "POST":
			f.note("external", r)
			w.Write([]byte(`{"id":"ba_plaid","last4":"0000","routing_number":"021000021"}`))
		case p == "/v1/transfers" && r.Method == "POST":
			f.note("transfer", r)
			w.Write([]byte(`{"id":"tr_advance"}`))
		case p == "/v1/payouts" && r.Method == "POST":
			f.note("payout", r)
			f.mu.Lock()
			fail := f.failPayouts > 0
			if fail {
				f.failPayouts--
			}
			f.mu.Unlock()
			if fail {
				http.Error(w, `{"error":{"message":"payout failed"}}`, http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"id":"po_advance","status":"pending"}`))
		case p == "/auth/get":
			w.Write([]byte(`{"numbers":{"ach":[{"account_id":"other","account":"999","routing":"111"},{"account_id":"chk","account":"1111222233330000","routing":"021000021"}]}}`))
		case p == "/v1/payouts":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"amount": 100000}, {"amount": 250000}}})
		case p == "/item/get":
			w.Write([]byte(`{"item":{"institution_id":"ins_109508"}}`))
		case p == "/accounts/get":
			if f.noChecking {
				w.Write([]byte(`{"accounts":[{"account_id":"other","subtype":"401k","mask":"6666","balances":{"current":1}}]}`))
				return
			}
			w.Write([]byte(`{"accounts":[{"account_id":"chk","subtype":"checking","mask":"0000","balances":{"current":110}}]}`))
		case p == "/transactions/get":
			w.Write([]byte(`{"total_transactions":2}`))
		case p == "/api.xro/2.0/Accounts" && r.Method == "GET":
			f.mu.Lock()
			list := []map[string]string{}
			for _, c := range f.xeroAccounts {
				list = append(list, map[string]string{"Code": c})
			}
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"Accounts": list})
		case p == "/api.xro/2.0/Accounts" && r.Method == "PUT":
			var in struct{ Code string }
			json.Unmarshal(f.noteJSON("account", r), &in)
			f.mu.Lock()
			f.xeroAccounts = append(f.xeroAccounts, in.Code)
			f.mu.Unlock()
			w.Write([]byte(`{"Accounts":[{"Code":"` + in.Code + `"}]}`))
		case p == "/api.xro/2.0/Contacts" && r.Method == "GET":
			f.mu.Lock()
			list := append([]map[string]string{}, f.xeroContacts...)
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"Contacts": list})
		case p == "/api.xro/2.0/Contacts" && r.Method == "PUT":
			var in struct{ Contacts []struct{ Name string } }
			json.Unmarshal(f.noteJSON("contact", r), &in)
			c := map[string]string{"ContactID": "c-lender", "Name": in.Contacts[0].Name}
			f.mu.Lock()
			f.xeroContacts = append(f.xeroContacts, c)
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"Contacts": []map[string]string{c}})
		case p == "/api.xro/2.0/BankTransactions" && r.Method == "GET":
			f.mu.Lock()
			list := append([]map[string]string{}, f.bankTxs...)
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"BankTransactions": list})
		case p == "/api.xro/2.0/BankTransactions" && r.Method == "PUT":
			var in struct{ BankTransactions []struct{ Reference string } }
			body := f.noteJSON("banktx", r)
			json.Unmarshal(body, &in)
			f.mu.Lock()
			fail := f.failBankTx > 0
			if fail {
				f.failBankTx--
			}
			f.mu.Unlock()
			if fail {
				http.Error(w, `{"Title":"boom"}`, http.StatusInternalServerError)
				return
			}
			tx := map[string]string{"BankTransactionID": "bt_advance", "Reference": in.BankTransactions[0].Reference}
			f.mu.Lock()
			f.bankTxs = append(f.bankTxs, tx)
			f.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"BankTransactions": []map[string]string{tx}})
		case p == "/api.xro/2.0/Organisation":
			w.Write([]byte(`{"Organisations":[{"Name":"Demo Company (Global)"}]}`))
		case p == "/api.xro/2.0/Invoices":
			day := func(n int) time.Time { return asOf.AddDate(0, 0, n) }
			inv := func(status string, total, due float64, dueDate time.Time) map[string]any {
				return map[string]any{"Type": "ACCREC", "Status": status, "Total": total, "AmountDue": due, "DueDate": asXeroDate(dueDate)}
			}
			json.NewEncoder(w).Encode(map[string]any{"Invoices": []map[string]any{
				inv("PAID", 6200, 0, day(-48)), inv("PAID", 4850, 0, day(-34)), inv("PAID", 7300, 0, day(-19)),
				inv("AUTHORISED", 5400, 5400, day(-3)), // overdue
				inv("AUTHORISED", 3950, 3950, day(9)),
				inv("AUTHORISED", 6800, 6800, day(21)),
				{"Type": "ACCPAY", "Status": "AUTHORISED", "Total": 1000, "AmountDue": 1000, "DueDate": asXeroDate(day(-10))}, // a bill, not a sale
				inv("VOIDED", 777, 777, day(-5)),
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeVendors) vendors(rec *wire.Recorder) Vendors {
	return NewVendors(rec, world.Targets{
		Stripe: wire.Direct("stripe", f.srv.URL),
		Plaid:  wire.Direct("plaid", f.srv.URL),
		Xero:   wire.Direct("xero", f.srv.URL),
	})
}

func TestReadStripeAddsUpTheWindowsFromTheCharges(t *testing.T) {
	f := newFakeVendors(t)
	got, err := f.vendors(wire.NewRecorder(2000)).readStripe(context.Background(), facts())
	if err != nil {
		t.Fatal(err)
	}

	var want StripeSignals
	for _, o := range world.Orders() {
		want.Sales90Cents += o.Cents
		want.Orders90++
		switch {
		case o.Day >= world.Days-30:
			want.Sales30Cents += o.Cents
			want.Orders30++
		case o.Day >= world.Days-60:
			want.SalesPrev30Cents += o.Cents
		}
	}
	want.Payouts, want.PayoutCents = 2, 350000
	if got != want {
		t.Fatalf("signals differ from the world's orders:\n got %+v\nwant %+v", got, want)
	}
	if got.Sales30Cents <= got.SalesPrev30Cents {
		t.Fatalf("the world grows, so the last 30 days (%d) should beat the 30 before (%d)", got.Sales30Cents, got.SalesPrev30Cents)
	}
	if calls := f.chargeCalls.Load(); calls != 10 {
		t.Fatalf("941 charges at 100 a page is 10 requests, got %d", calls)
	}
}

func TestReadXeroCountsOnlySalesInvoicesAndFindsTheOverdueOne(t *testing.T) {
	f := newFakeVendors(t)
	got, err := f.vendors(wire.NewRecorder(10)).readXero(context.Background(), facts())
	if err != nil {
		t.Fatal(err)
	}
	want := XeroSignals{Invoices: 6, BilledCents: 3450000, OutstandingCents: 1615000, OverdueCents: 540000}
	if got != want {
		t.Fatalf("got %+v want %+v (bills and voided invoices must not count)", got, want)
	}
}

func TestReadPlaidVerifiesTheAccountAndRecordsTheThinData(t *testing.T) {
	f := newFakeVendors(t)
	got, err := f.vendors(wire.NewRecorder(10)).readPlaid(context.Background(), facts())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Verified || got.Mask != "0000" || got.BalanceCents != 11000 || got.Transactions != 2 || got.Institution != "ins_109508" {
		t.Fatalf("unexpected signals: %+v", got)
	}
}

func TestConnectChecksEachAccountAndSaysWhatItFound(t *testing.T) {
	f := newFakeVendors(t)
	v := f.vendors(wire.NewRecorder(10))
	ctx := context.Background()
	for vendor, want := range map[string]string{"stripe": "acct_m", "plaid": "ending 0000", "xero": "Demo Company (Global)"} {
		var c Connection
		var err error
		switch vendor {
		case "stripe":
			c, err = v.connectStripe(ctx, facts())
		case "plaid":
			c, err = v.connectPlaid(ctx, facts())
		case "xero":
			c, err = v.connectXero(ctx, facts())
		}
		if err != nil || c.Vendor != vendor || !strings.Contains(c.Detail, want) {
			t.Errorf("%s: %+v, %v (want detail containing %q)", vendor, c, err, want)
		}
	}
}

func TestConnectPlaidFailsWhenTheAccountIsNotOnTheConnection(t *testing.T) {
	f := newFakeVendors(t)
	f.noChecking = true
	_, err := f.vendors(wire.NewRecorder(10)).connectPlaid(context.Background(), facts())
	if err == nil || !strings.Contains(err.Error(), "no account chk") {
		t.Fatalf("want a clear error: %v", err)
	}
}

// Every call goes through the proxy, and a credential never reaches the recording.
func TestEveryVendorCallIsRecordedAndCredentialsAreMasked(t *testing.T) {
	f := newFakeVendors(t)
	rec := wire.NewRecorder(2000)
	v := f.vendors(rec)
	ctx := context.Background()
	v.connectPlaid(ctx, facts())
	v.connectXero(ctx, facts())
	v.readXero(ctx, facts())

	evs := rec.Events()
	if len(evs) != 4 { // item/get, accounts/get, Organisation, Invoices
		t.Fatalf("want 4 recorded calls, got %d", len(evs))
	}
	vendors := map[string]int{}
	for _, e := range evs {
		vendors[e.Vendor]++
		if e.Source != wire.SourceLocal {
			t.Errorf("event %d should say who served it: %q", e.ID, e.Source)
		}
		if strings.Contains(e.Request.Content, "access-secret-token") || strings.Contains(e.Request.Content, "acme-lender-secret") {
			t.Errorf("a credential leaked into the recording of %s %s: %s", e.Method, e.Path, e.Request.Content)
		}
	}
	if vendors["plaid"] != 2 || vendors["xero"] != 2 {
		t.Fatalf("calls by vendor: %v", vendors)
	}
}

func TestParseXeroDate(t *testing.T) {
	want := time.Date(2027, 1, 29, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{
		fmt.Sprintf("/Date(%d+0000)/", want.UnixMilli()),
		fmt.Sprintf("/Date(%d)/", want.UnixMilli()),
		"2027-01-29T00:00:00",
		"2027-01-29",
	} {
		if got, err := parseXeroDate(in); err != nil || !got.Equal(want) {
			t.Errorf("parseXeroDate(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseXeroDate("next Tuesday"); err == nil {
		t.Error("an unrecognised date must be an error")
	}
}

func TestSwitchedVendorsStayOnTheEmulatorAndRecordWhereADryRunWouldGo(t *testing.T) {
	f := newFakeVendors(t)
	rec := wire.NewRecorder(10)
	sw := wire.NewSwitch(
		wire.Lane{Vendor: "stripe", Name: "Stripe", Emulator: wire.Direct("stripe", f.srv.URL), ServiceURL: "https://api.stripe.com"},
		wire.Lane{Vendor: "plaid", Name: "Plaid", Emulator: wire.Direct("plaid", f.srv.URL), ServiceURL: "https://production.plaid.com"},
		wire.Lane{Vendor: "xero", Name: "Xero", Emulator: wire.Direct("xero", f.srv.URL), ServiceURL: "https://api.xero.com"},
	)
	v := NewSwitchedVendors(rec, sw)
	ctx := context.Background()
	last := func() wire.Event { evs := rec.Events(); return evs[len(evs)-1] }

	if _, err := v.connectXero(ctx, facts()); err != nil || last().WouldSend != "" {
		t.Fatalf("at the emulator: %v, %+v", err, last())
	}

	// Pointed at Xero's own service, the call is still answered by the app
	// emulator, and the recording says where it would have gone.
	if err := sw.Apply(map[string]string{"xero": "https://api.xero.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.connectXero(ctx, facts()); err != nil {
		t.Fatalf("a dry run must still work: %v", err)
	}
	if got := last(); got.WouldSend != "https://api.xero.com" || got.Origin != wire.OriginEmulator || got.Status != 200 {
		t.Fatalf("recorded %+v", got)
	}

	// An invalid URL fails with a message that says what to do.
	_ = sw.Apply(map[string]string{"xero": "xero"})
	if _, err := v.connectXero(ctx, facts()); err == nil || !strings.Contains(err.Error(), "Use emulator") {
		t.Fatalf("an invalid URL: %v", err)
	}

	// Back at the emulator, nothing is recorded as a dry run.
	_ = sw.Apply(map[string]string{"xero": f.srv.URL})
	if _, err := v.connectXero(ctx, facts()); err != nil || last().WouldSend != "" {
		t.Fatalf("back at the emulator: %v, %+v", err, last())
	}
}
