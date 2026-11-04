package wire

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientServesCallsThroughTheProxyWithNoListener(t *testing.T) {
	var gotHost, gotPath, gotBody string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotHost, gotPath, gotBody = r.Host, r.URL.RequestURI(), string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer up.Close()

	rec := NewRecorder(10)
	px := NewProxy(Target{Vendor: "stripe", Source: SourceLocalStack, Upstream: up.URL, Host: "stripe.localhost.localstack.cloud:4566"}, rec)
	c := px.Client()

	// The host in the URL is a placeholder; the proxy uses its own target.
	resp, err := c.Post("http://wire.invalid/v1/charges?limit=2", "application/json", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 || string(body) != `{"ok":true}` {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if gotHost != "stripe.localhost.localstack.cloud:4566" || gotPath != "/v1/charges?limit=2" || gotBody != `{"a":1}` {
		t.Fatalf("upstream saw host=%q path=%q body=%q", gotHost, gotPath, gotBody)
	}
	evs := rec.Events()
	if len(evs) != 1 || evs[0].Vendor != "stripe" || evs[0].Source != SourceLocalStack || evs[0].Request.Content != `{"a":1}` {
		t.Fatalf("the call should be recorded: %+v", evs)
	}
}

func TestClientHandlesARequestWithNoBody(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("{}")) }))
	defer up.Close()
	c := NewProxy(Direct("xero", up.URL), NewRecorder(10)).Client()
	resp, err := c.Get("http://wire.invalid/api.xro/2.0/Invoices") // a GET has a nil body on the client side
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestClientReportsAnUnreachableUpstreamAsA502NotAnError(t *testing.T) {
	up := httptest.NewServer(http.NotFoundHandler())
	url := up.URL
	up.Close()
	rec := NewRecorder(10)
	resp, err := NewProxy(Direct("plaid", url), rec).Client().Get("http://wire.invalid/item/get")
	if err != nil {
		t.Fatalf("a failed upstream is an HTTP answer, not a transport error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || rec.Events()[0].Error == "" {
		t.Fatalf("status=%d events=%+v", resp.StatusCode, rec.Events())
	}
}
