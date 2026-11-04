package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// upstream is an app emulator that echoes what it was asked.
func upstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"path":"`+r.URL.Path+`"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func startProxy(t *testing.T, cfg config) *instance {
	t.Helper()
	inst, err := start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inst.shutdown(context.Background()) })
	return inst
}

func get(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestARecordedCallIsForwardedAndServedBack(t *testing.T) {
	up := upstream(t)
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}},
		events:  "127.0.0.1:0",
		keep:    100,
		getenv: env(map[string]string{
			"STRIPE_EMULATOR_URL":     up.URL,
			"STRIPE_EMULATOR_VERSION": "0123456789ab",
		}),
	})

	if got := get(t, "http://"+inst.addrs["stripe"]+"/v1/charges?limit=1"); got != `{"path":"/v1/charges"}` {
		t.Fatalf("the call was not forwarded: %q", got)
	}

	var rec recording
	if err := json.Unmarshal([]byte(get(t, "http://"+inst.eventsAddr+"/events")), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Events) != 1 {
		t.Fatalf("want one recorded call, got %+v", rec)
	}
	ev := rec.Events[0]
	if ev.Vendor != "stripe" || ev.Method != "GET" || ev.Path != "/v1/charges?limit=1" || ev.Status != 200 ||
		ev.Source != "local" || ev.Origin != "emulator" || ev.EmulatorVersion != "0123456789ab" {
		t.Fatalf("unexpected event: %+v", ev)
	}

	// since returns only what is new, and keeps the cursor when nothing is.
	if err := json.Unmarshal([]byte(get(t, "http://"+inst.eventsAddr+"/events?since=1")), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Events) != 0 || rec.Last != 1 {
		t.Fatalf("since=1 should be empty with the cursor kept: %+v", rec)
	}
}

func TestVendorsShareOneRecording(t *testing.T) {
	up := upstream(t)
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}, {"plaid", "127.0.0.1:0"}},
		keep:    100,
		getenv:  env(map[string]string{"STRIPE_EMULATOR_URL": up.URL, "PLAID_EMULATOR_URL": up.URL}),
	})
	if inst.addrs["stripe"] == inst.addrs["plaid"] {
		t.Fatal("each vendor needs its own port")
	}
	get(t, "http://"+inst.addrs["stripe"]+"/v1/a")
	get(t, "http://"+inst.addrs["plaid"]+"/item/get")

	evs := inst.rec.Events()
	if len(evs) != 2 || evs[0].Vendor != "stripe" || evs[1].Vendor != "plaid" || evs[1].ID != 2 {
		t.Fatalf("want one ordered recording of both vendors: %+v", evs)
	}
}

// The proxy is the same one the demo uses, so credentials in a body are masked
// before they are recorded or written.
func TestCredentialsAreMaskedInTheRecording(t *testing.T) {
	up := upstream(t)
	out := filepath.Join(t.TempDir(), "calls.json")
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}},
		out:     out,
		keep:    100,
		getenv:  env(map[string]string{"STRIPE_EMULATOR_URL": up.URL}),
	})
	resp, err := http.Post("http://"+inst.addrs["stripe"]+"/v1/x", "application/json", strings.NewReader(`{"account_number":"000123456789"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if err := inst.stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "000123456789") {
		t.Fatalf("the account number reached the recording:\n%s", b)
	}
}

func TestStopWritesTheRecordingToOut(t *testing.T) {
	up := upstream(t)
	out := filepath.Join(t.TempDir(), "nested", "calls.json")
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}},
		out:     out,
		keep:    100,
		getenv:  env(map[string]string{"STRIPE_EMULATOR_URL": up.URL}),
	})
	get(t, "http://"+inst.addrs["stripe"]+"/v1/a")
	get(t, "http://"+inst.addrs["stripe"]+"/v1/b")
	if err := inst.stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	var rec recording
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("the file is not a recording: %v", err)
	}
	if len(rec.Events) != 2 || rec.Last != 2 {
		t.Fatalf("want both calls in the file: %+v", rec)
	}
	if entries, _ := os.ReadDir(filepath.Dir(out)); len(entries) != 1 {
		t.Fatalf("a temporary file was left behind: %v", entries)
	}
}

func TestStopWithoutOutWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	up := upstream(t)
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}},
		keep:    100,
		getenv:  env(map[string]string{"STRIPE_EMULATOR_URL": up.URL}),
	})
	if err := inst.stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("wrote files nobody asked for: %v", entries)
	}
}

func TestAnUnreachableAppEmulatorIsRecordedAsAFailedCall(t *testing.T) {
	inst := startProxy(t, config{
		proxies: []proxySpec{{"xero", "127.0.0.1:0"}},
		keep:    100,
		getenv:  env(map[string]string{"XERO_EMULATOR_URL": "http://127.0.0.1:1"}),
	})
	resp, err := http.Get("http://" + inst.addrs["xero"] + "/x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", resp.StatusCode)
	}
	evs := inst.rec.Events()
	if len(evs) != 1 || evs[0].Error == "" {
		t.Fatalf("the failure should be recorded: %+v", evs)
	}
}

func TestBadEventsCursorIs400(t *testing.T) {
	up := upstream(t)
	inst := startProxy(t, config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}},
		events:  "127.0.0.1:0",
		keep:    100,
		getenv:  env(map[string]string{"STRIPE_EMULATOR_URL": up.URL}),
	})
	resp, err := http.Get("http://" + inst.eventsAddr + "/events?since=abc")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestStartFailsAndWritesNothingWhenAPortIsTaken(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	out := filepath.Join(t.TempDir(), "calls.json")
	_, err = start(config{
		proxies: []proxySpec{{"stripe", "127.0.0.1:0"}, {"plaid", taken.Addr().String()}},
		out:     out,
		keep:    100,
		getenv:  env(nil),
	})
	if err == nil || !strings.Contains(err.Error(), "plaid") {
		t.Fatalf("want an error naming plaid, got %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("a failed start must not write the recording")
	}
}

func TestRunRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no proxy", nil, "usage:"},
		{"a proxy without an address", []string{"--proxy", "stripe"}, "vendor=addr"},
		{"the same vendor twice", []string{"--proxy", "stripe=127.0.0.1:0", "--proxy", "stripe=127.0.0.1:0"}, "twice"},
		{"a stray argument", []string{"--proxy", "stripe=127.0.0.1:0", "extra"}, "usage:"},
		{"keep must be positive", []string{"--proxy", "stripe=127.0.0.1:0", "--keep", "0"}, "usage:"},
		{"an unknown flag", []string{"--nope"}, "usage:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(context.Background(), tc.args, env(nil), &out, &errb); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
			if !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("stderr %q should contain %q", errb.String(), tc.want)
			}
		})
	}
}

func TestRunServesUntilStoppedThenWritesOut(t *testing.T) {
	up := upstream(t)
	out := filepath.Join(t.TempDir(), "calls.json")
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--proxy", "stripe=127.0.0.1:0", "--out", out}, env(map[string]string{"STRIPE_EMULATOR_URL": up.URL}), &stdout, &stderr)
	}()

	var addr string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// The first line is "stripe  http://ADDR -> local URL".
		if f := strings.Fields(stdout.String()); len(f) >= 2 {
			addr = strings.TrimPrefix(f[1], "http://")
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatalf("never announced where it listens; stderr: %s", stderr.String())
	}
	get(t, "http://"+addr+"/v1/a")
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("did not stop")
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"/v1/a"`) {
		t.Fatalf("recording not written: %v\n%s", err, b)
	}
	if !strings.Contains(stdout.String(), "wrote 1 calls to") {
		t.Fatalf("stdout: %s", stdout.String())
	}
}
