package wire

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// upstream returns a stand-in for LocalStack's gateway and the Host header and
// body it last received.
func upstream(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *struct{ host, body string }) {
	t.Helper()
	seen := &struct{ host, body string }{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.host, seen.body = r.Host, string(b)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, seen
}

func TestProxyForwardsWithHostHeaderAndRecords(t *testing.T) {
	up, seen := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"in_1"}`))
	})
	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Target{Vendor: "stripe", Upstream: up.URL, Host: "stripe.localhost.localstack.cloud:4566"}, rec))
	defer px.Close()

	resp, err := http.Post(px.URL+"/v1/invoices?limit=3", "application/json", strings.NewReader(`{"customer":"cus_1"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated || string(body) != `{"id":"in_1"}` {
		t.Fatalf("caller got %d %q", resp.StatusCode, body)
	}
	if seen.host != "stripe.localhost.localstack.cloud:4566" {
		t.Fatalf("upstream saw Host %q, want the LocalStack gateway host name", seen.host)
	}
	if seen.body != `{"customer":"cus_1"}` {
		t.Fatalf("upstream saw body %q", seen.body)
	}

	evs := rec.Events()
	if len(evs) != 1 {
		t.Fatalf("recorded %d events, want 1", len(evs))
	}
	e := evs[0]
	if e.Vendor != "stripe" || e.Method != "POST" || e.Path != "/v1/invoices?limit=3" || e.Status != 201 {
		t.Fatalf("unexpected event: %+v", e)
	}
	if e.Request.Content != `{"customer":"cus_1"}` || e.Response.Content != `{"id":"in_1"}` {
		t.Fatalf("bodies not recorded: %+v / %+v", e.Request, e.Response)
	}
	if e.Duration <= 0 {
		t.Fatalf("duration not recorded: %v", e.Duration)
	}
}

func TestProxyRedactsCredentialsInRecordedBodiesOnly(t *testing.T) {
	up, seen := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"tok_live","item_id":"item_1"}`))
	})
	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Target{Vendor: "plaid", Upstream: up.URL}, rec))
	defer px.Close()

	resp, err := http.Post(px.URL+"/item/get", "application/json", strings.NewReader(`{"client_id":"cid","secret":"s3cret"}`))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// The vendor and the caller still get the real values.
	if seen.body != `{"client_id":"cid","secret":"s3cret"}` {
		t.Fatalf("upstream body was altered: %q", seen.body)
	}
	if !strings.Contains(string(got), "tok_live") {
		t.Fatalf("caller body was altered: %q", got)
	}
	// The recording does not.
	e := rec.Events()[0]
	for _, b := range []Body{e.Request, e.Response} {
		if strings.Contains(b.Content, "s3cret") || strings.Contains(b.Content, "tok_live") {
			t.Fatalf("credential leaked into the recording: %q", b.Content)
		}
	}
	if !strings.Contains(e.Request.Content, "cid") || !strings.Contains(e.Response.Content, "item_1") {
		t.Fatalf("non-secret fields should survive: %+v / %+v", e.Request, e.Response)
	}
}

func TestProxyForwardsLargeBodiesInFullButRecordsTruncated(t *testing.T) {
	big := strings.Repeat("a", 100<<10)
	up, _ := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(big))
	})
	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Target{Vendor: "xero", Upstream: up.URL}, rec))
	defer px.Close()

	resp, err := http.Get(px.URL + "/Invoices")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if len(got) != len(big) {
		t.Fatalf("caller got %d bytes, want %d", len(got), len(big))
	}
	e := rec.Events()[0]
	if !e.Response.Truncated || e.Response.Size != len(big) || len(e.Response.Content) != maxRecorded {
		t.Fatalf("unexpected recording: truncated=%v size=%d kept=%d", e.Response.Truncated, e.Response.Size, len(e.Response.Content))
	}
}

func TestProxyReturns502AndRecordsWhenUpstreamIsDown(t *testing.T) {
	up := httptest.NewServer(http.NotFoundHandler())
	url := up.URL
	up.Close() // nothing listens any more

	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Target{Vendor: "stripe", Upstream: url}, rec))
	defer px.Close()

	resp, err := http.Get(px.URL + "/v1/balance")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"vendor":"stripe"`) {
		t.Fatalf("error body should name the vendor: %s", body)
	}
	e := rec.Events()[0]
	if e.Status != http.StatusBadGateway || e.Error == "" {
		t.Fatalf("failed call not recorded: %+v", e)
	}
}

func TestProxyDoesNotFollowRedirects(t *testing.T) {
	up, _ := upstream(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	})
	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Target{Vendor: "xero", Upstream: up.URL}, rec))
	defer px.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(px.URL + "/connect")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/elsewhere" {
		t.Fatalf("redirect should reach the caller untouched: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestLocalStackTarget(t *testing.T) {
	got := LocalStack("plaid")
	if got.Upstream != "http://127.0.0.1:4566" || got.Host != "plaid.localhost.localstack.cloud:4566" || got.Vendor != "plaid" {
		t.Fatalf("unexpected target: %+v", got)
	}
}

func TestDirectTargetKeepsTheHostHeaderAndTrimsTheSlash(t *testing.T) {
	got := Direct("plaid", "http://127.0.0.1:5101/")
	if got.Upstream != "http://127.0.0.1:5101" || got.Host != "" || got.Source != SourceLocal || got.Vendor != "plaid" {
		t.Fatalf("unexpected target: %+v", got)
	}
}

// The switch from a local app emulator to LocalStack is one environment
// variable and no code change.
func TestTargetFromEnvChoosesLocalOrLocalStack(t *testing.T) {
	env := map[string]string{"PLAID_EMULATOR_URL": "http://127.0.0.1:5101"}
	get := func(k string) string { return env[k] }

	plaid := TargetFromEnv(get, "plaid")
	if plaid.Source != SourceLocal || plaid.Upstream != "http://127.0.0.1:5101" || plaid.Host != "" {
		t.Fatalf("plaid should run locally: %+v", plaid)
	}
	stripe := TargetFromEnv(get, "stripe")
	if stripe.Source != SourceLocalStack || stripe.Host != "stripe.localhost.localstack.cloud:4566" {
		t.Fatalf("stripe should use LocalStack: %+v", stripe)
	}

	delete(env, "PLAID_EMULATOR_URL") // LocalStack now includes Plaid
	if got := TargetFromEnv(get, "plaid"); got.Source != SourceLocalStack {
		t.Fatalf("plaid should have moved to LocalStack: %+v", got)
	}
}

func TestEventsSayWhoHostsTheAppEmulator(t *testing.T) {
	up, _ := upstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{}")) })
	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(Direct("xero", up.URL), rec))
	defer px.Close()
	resp, err := http.Get(px.URL + "/Invoices")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := rec.Events()[0].Source; got != SourceLocal {
		t.Fatalf("Source = %q, want %q", got, SourceLocal)
	}
}

// A recorded call says who produced the response and which version of it, so
// the same call made against the vendor can be compared with it later.
func TestEventsCarryOriginAndVersions(t *testing.T) {
	up, _ := upstream(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{}")) })

	t.Run("an app emulator by default, with no version known", func(t *testing.T) {
		rec := NewRecorder(10)
		px := httptest.NewServer(NewProxy(Direct("xero", up.URL), rec))
		defer px.Close()
		http.Get(px.URL + "/x")
		ev := rec.Events()[0]
		if ev.Origin != OriginEmulator || ev.EmulatorVersion != "" || ev.ArtifactVersion != "" {
			t.Fatalf("unexpected provenance: %+v", ev)
		}
	})

	t.Run("what the target says", func(t *testing.T) {
		rec := NewRecorder(10)
		tg := Direct("xero", up.URL)
		tg.Origin, tg.EmulatorVersion, tg.ArtifactVersion = OriginVendor, "0123456789ab", "2.0"
		px := httptest.NewServer(NewProxy(tg, rec))
		defer px.Close()
		http.Get(px.URL + "/x")
		ev := rec.Events()[0]
		if ev.Origin != OriginVendor || ev.EmulatorVersion != "0123456789ab" || ev.ArtifactVersion != "2.0" {
			t.Fatalf("unexpected provenance: %+v", ev)
		}
	})

	t.Run("a call that could not be made still says where it was going", func(t *testing.T) {
		rec := NewRecorder(10)
		tg := Direct("xero", "http://127.0.0.1:1") // nothing listens here
		tg.EmulatorVersion = "0123456789ab"
		px := httptest.NewServer(NewProxy(tg, rec))
		defer px.Close()
		http.Get(px.URL + "/x")
		ev := rec.Events()[0]
		if ev.Error == "" || ev.Origin != OriginEmulator || ev.EmulatorVersion != "0123456789ab" {
			t.Fatalf("a failed call lost its provenance: %+v", ev)
		}
	})
}

func TestTargetFromEnvReadsTheEmulatorVersion(t *testing.T) {
	env := map[string]string{
		"PLAID_EMULATOR_URL":      "http://127.0.0.1:5101",
		"PLAID_EMULATOR_VERSION":  "aaaaaaaaaaaa",
		"STRIPE_EMULATOR_VERSION": "bbbbbbbbbbbb",
	}
	get := func(k string) string { return env[k] }
	if got := TargetFromEnv(get, "plaid"); got.EmulatorVersion != "aaaaaaaaaaaa" {
		t.Fatalf("a local app emulator: %+v", got)
	}
	if got := TargetFromEnv(get, "stripe"); got.EmulatorVersion != "bbbbbbbbbbbb" || got.Source != SourceLocalStack {
		t.Fatalf("one hosted by LocalStack: %+v", got)
	}
	if got := TargetFromEnv(get, "xero"); got.EmulatorVersion != "" {
		t.Fatalf("an unset version is unknown, not guessed: %+v", got)
	}
}
