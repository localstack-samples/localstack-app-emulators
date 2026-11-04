package gaps

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// The three manifest formats seen in WonderTwin-AI/wondertwin.
var (
	xero   = ParseManifest([]string{"GET /api.xro/2.0/Accounts", "PUT /api.xro/2.0/Accounts", "GET /api.xro/2.0/Invoices/{}"})
	plaid  = ParseManifest([]string{"item/get", "sandbox/public_token/create"})
	stripe = ParseManifest([]string{"customers", "payment_intents", "external_accounts"})
)

func TestCoversByRoute(t *testing.T) {
	tests := []struct {
		method, path string
		covered      bool
	}{
		{"GET", "/api.xro/2.0/Accounts", true},
		{"PUT", "/api.xro/2.0/Accounts", true},
		{"POST", "/api.xro/2.0/Accounts", false}, // same path, a method the manifest lacks
		{"GET", "/api.xro/2.0/Invoices/8d1f1b5e-35b4-4e4d-9f8e-0d8b3f6a7c11", true},
		{"GET", "/api.xro/2.0/Invoices", false}, // the list is a different route from one invoice
		{"GET", "/api.xro/2.0/NotAThing", false},
	}
	for _, tc := range tests {
		got, basis := xero.Covers(tc.method, tc.path)
		if got != tc.covered || basis != BasisRoute {
			t.Errorf("%s %s: covered=%v basis=%q, want covered=%v basis=route", tc.method, tc.path, got, basis, tc.covered)
		}
	}
}

func TestCoversByPathIgnoresTheMethod(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		if got, basis := plaid.Covers(method, "/item/get"); !got || basis != BasisPath {
			t.Errorf("%s /item/get: covered=%v basis=%q", method, got, basis)
		}
	}
	if got, basis := plaid.Covers("POST", "/accounts/get"); got || basis != BasisPath {
		t.Errorf("an unlisted path should be an uncovered path-level answer: covered=%v basis=%q", got, basis)
	}
}

func TestCoversByResourceName(t *testing.T) {
	tests := []struct {
		path    string
		covered bool
	}{
		{"/v1/payment_intents", true},
		{"/v1/payment_intents/pi_3Nq8Z2LkdIwHu7ix", true},
		{"/v1/payment_intents/pi_3Nq8Z2LkdIwHu7ix/confirm", true},
		{"/v1/customers", true},
		// The last resource counts, so a nested one is found by its own name.
		{"/v1/accounts/acct_1Nq8Z2LkdIwHu7ix/external_accounts", true},
		{"/v1/radar/value_lists", false},
		{"/v1/refunds", false},
	}
	for _, tc := range tests {
		got, basis := stripe.Covers("GET", tc.path)
		if got != tc.covered || basis != BasisResource {
			t.Errorf("%s: covered=%v basis=%q, want covered=%v basis=resource", tc.path, got, basis, tc.covered)
		}
	}
}

func TestResourceNamesDoNotMistakeAnUnderscoredNameForAnID(t *testing.T) {
	// "payment_intents" has the shape of a prefixed ID, but IDs carry a digit or
	// a capital and resource names are lower-case words.
	for _, name := range []string{"payment_intents", "external_accounts", "financial_connections", "public_token"} {
		if idLike(name) {
			t.Errorf("%q is a resource name, not an ID", name)
		}
	}
	for _, id := range []string{"acct_3BsYdLdigjpvPgKzcNdCWOQl", "ch_q7BU0ZeG73uY827vdkPgZAap", "8d1f1b5e-35b4-4e4d-9f8e-0d8b3f6a7c11", "12345", "pi_abc123"} {
		if !idLike(id) {
			t.Errorf("%q is an ID", id)
		}
	}
}

func TestNormalizeReplacesIdentifiers(t *testing.T) {
	tests := map[string]string{
		"/v1/charges": "/v1/charges",
		"/v1/accounts/acct_3BsYdLdigjpvPgKzcNdCWOQl":                 "/v1/accounts/{}",
		"/api.xro/2.0/Invoices/8d1f1b5e-35b4-4e4d-9f8e-0d8b3f6a7c11": "/api.xro/2.0/Invoices/{}",
		"/item/get": "/item/get",
	}
	for in, want := range tests {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func ev(vendor, method, path string, status int, version string) wire.Event {
	return wire.Event{Vendor: vendor, Method: method, Path: path, Status: status, EmulatorVersion: version}
}

func TestBuildGroupsCallsAndMarksGaps(t *testing.T) {
	events := []wire.Event{
		ev("xero", "PUT", "/api.xro/2.0/Accounts", 200, "aaaaaaaaaaaa"),
		ev("xero", "POST", "/api.xro/2.0/Accounts", 405, "aaaaaaaaaaaa"),
		ev("stripe", "GET", "/v1/charges?limit=100", 200, ""),
		ev("stripe", "GET", "/v1/charges?limit=100&starting_after=ch_q7BU0ZeG73uY827vdkPgZAap", 200, ""),
		ev("stripe", "GET", "/v1/accounts/acct_3BsYdLdigjpvPgKzcNdCWOQl", 200, ""),
		ev("stripe", "GET", "/v1/accounts/acct_9ZxYdLdigjpvPgKzcNdCWOQl", 404, ""),
		ev("plaid", "POST", "/item/get", 200, ""), // no plaid manifest was given
	}
	r := Build(events, map[string]Manifest{"xero": xero, "stripe": stripe})

	got := map[string]Call{}
	for _, c := range r.Calls {
		got[c.Vendor+":"+c.Method+" "+c.Path] = c
	}
	// Two charge reads differ only by cursor and query, so they are one call.
	if c := got["stripe:GET /v1/charges"]; c.Count != 2 || c.Coverage != Gap {
		t.Errorf("charges: %+v", c)
	}
	// Two accounts differ only by ID, and one of them failed.
	if c := got["stripe:GET /v1/accounts/{}"]; c.Count != 2 || c.Failed != 1 || c.Coverage != Gap {
		t.Errorf("accounts: %+v", c)
	}
	if c := got["xero:PUT /api.xro/2.0/Accounts"]; c.Coverage != Covered || c.Basis != BasisRoute {
		t.Errorf("a covered route: %+v", c)
	}
	if c := got["xero:POST /api.xro/2.0/Accounts"]; c.Coverage != Gap || c.Failed != 1 {
		t.Errorf("a method the manifest lacks: %+v", c)
	}
	if c := got["plaid:POST /item/get"]; c.Coverage != Unchecked || c.Basis != BasisNone {
		t.Errorf("a vendor with no manifest must be unchecked, not a gap: %+v", c)
	}
	// customers and payment_intents are listed, but charges and accounts are not.
	if r.Gaps() != 3 {
		t.Errorf("want 3 gaps, got %d: %+v", r.Gaps(), r.Calls)
	}
	// Gaps come first.
	if r.Calls[0].Coverage != Gap || r.Calls[len(r.Calls)-1].Coverage == Gap {
		t.Errorf("gaps should sort first: %+v", r.Calls)
	}
	if v := r.Versions["xero"]; len(v) != 1 || v[0] != "aaaaaaaaaaaa" {
		t.Errorf("xero versions: %v", v)
	}
	if v := r.Versions["stripe"]; len(v) != 1 || v[0] != "unknown" {
		t.Errorf("a missing version is reported as unknown: %v", v)
	}
}

func TestAnErrorCountsAsFailedEvenWithoutAStatus(t *testing.T) {
	e := wire.Event{Vendor: "xero", Method: "GET", Path: "/api.xro/2.0/Accounts", Error: "connection refused"}
	r := Build([]wire.Event{e}, map[string]Manifest{"xero": xero})
	if r.Calls[0].Failed != 1 {
		t.Fatalf("%+v", r.Calls[0])
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	m, err := LoadManifest(write("ok.json", `{"coverage":{"resources_implemented":["GET /a/{}"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Covers("GET", "/a/1"); !got {
		t.Fatal("the loaded manifest should cover GET /a/1")
	}
	for name, body := range map[string]string{
		"empty.json":   `{"coverage":{"resources_implemented":[]}}`,
		"missing.json": `{}`,
		"bad.json":     `{nope`,
	} {
		if _, err := LoadManifest(write(name, body)); err == nil {
			t.Errorf("%s should be rejected: an empty manifest would make every call a gap", name)
		}
	}
	if _, err := LoadManifest(filepath.Join(dir, "absent.json")); err == nil {
		t.Error("a missing file should be an error")
	}
}

func TestWritePrintsGapsVersionsAndTheCaveat(t *testing.T) {
	r := Build([]wire.Event{
		ev("xero", "POST", "/api.xro/2.0/Accounts", 405, "aaaaaaaaaaaa"),
		ev("xero", "PUT", "/api.xro/2.0/Accounts", 200, "aaaaaaaaaaaa"),
	}, map[string]Manifest{"xero": xero})
	var b bytes.Buffer
	Write(&b, r)
	out := b.String()
	for _, want := range []string{"gap", "xero:POST /api.xro/2.0/Accounts", "covered", "2 distinct calls, 1 not covered", "xero app emulator: aaaaaaaaaaaa", "resource-level"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "gap") > strings.Index(out, "covered") {
		t.Errorf("gaps should be listed before covered calls:\n%s", out)
	}
}
