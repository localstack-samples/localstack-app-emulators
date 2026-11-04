//go:build live

package wire

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Run with: go test -tags live ./wire
//
// Needs LocalStack running on the default port with the Stripe app emulator.
// It proves the proxy reaches an app emulator through the gateway by Host
// header alone, with no DNS lookup of *.localhost.localstack.cloud.
func TestLiveStripeThroughLocalStackGateway(t *testing.T) {
	probe, err := http.Get(localStackEdge + "/_localstack/health")
	if err != nil {
		t.Skipf("LocalStack is not running on %s: %v", localStackEdge, err)
	}
	probe.Body.Close()

	rec := NewRecorder(10)
	px := httptest.NewServer(NewProxy(LocalStack("stripe"), rec))
	defer px.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(px.URL + "/admin/health")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Fatalf("got %d %q, want 200 {\"status\":\"ok\"}", resp.StatusCode, body)
	}
	evs := rec.Events()
	if len(evs) != 1 || evs[0].Vendor != "stripe" || evs[0].Status != 200 || strings.TrimSpace(evs[0].Response.Content) != `{"status":"ok"}` {
		t.Fatalf("unexpected recording: %+v", evs)
	}
}

// TestLiveDirectEmulators checks emulators you run yourself. Set, for example:
//
//	PLAID_EMULATOR_URL=http://127.0.0.1:5101 XERO_EMULATOR_URL=http://127.0.0.1:5102 \
//	  go test -tags live -run LiveDirect ./wire
//
// Each vendor whose variable is set must answer /admin/health through the proxy.
func TestLiveDirectEmulators(t *testing.T) {
	ran := 0
	for _, vendor := range []string{"stripe", "plaid", "xero"} {
		target := TargetFromEnv(os.Getenv, vendor)
		if target.Source != SourceLocal {
			continue
		}
		ran++
		rec := NewRecorder(10)
		px := httptest.NewServer(NewProxy(target, rec))
		resp, err := http.Get(px.URL + "/admin/health")
		if err != nil {
			t.Fatalf("%s: %v", vendor, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		px.Close()
		if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
			t.Fatalf("%s: got %d %q", vendor, resp.StatusCode, body)
		}
		if ev := rec.Events()[0]; ev.Source != SourceLocal || ev.Vendor != vendor {
			t.Fatalf("%s: unexpected event %+v", vendor, ev)
		}
		t.Logf("%s: healthy through the proxy at %s", vendor, target.Upstream)
	}
	if ran == 0 {
		t.Skip("set <VENDOR>_EMULATOR_URL for at least one vendor")
	}
}
