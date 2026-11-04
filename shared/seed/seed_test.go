package seed

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

func target(vendor string, srv *httptest.Server) wire.Target {
	return wire.Direct(vendor, srv.URL)
}

func TestAPISetsHostDecodesJSONAndReportsErrors(t *testing.T) {
	var gotHost, gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotHost, gotBody, gotType = r.Host, string(b), r.Header.Get("Content-Type")
		if r.URL.Path == "/boom" {
			http.Error(w, "  no such   thing  ", http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"id":"x1"}`))
	}))
	defer srv.Close()

	api := API{Target: wire.Target{Vendor: "stripe", Upstream: srv.URL, Host: "stripe.localhost.localstack.cloud:4566"}}
	var out struct{ ID string }
	if err := api.JSON(context.Background(), http.MethodPost, "/v1/x", nil, map[string]int{"n": 1}, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "x1" || gotHost != "stripe.localhost.localstack.cloud:4566" || gotType != "application/json" || gotBody != `{"n":1}` {
		t.Fatalf("got out=%+v host=%q type=%q body=%q", out, gotHost, gotType, gotBody)
	}

	err := api.Do(context.Background(), http.MethodGet, "/boom", nil, "", nil, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Body != "no such thing" || apiErr.Vendor != "stripe" {
		t.Fatalf("want a 404 *Error with a tidy body, got %v", err)
	}
}

func TestReadyClassifiesEachAppEmulator(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) }))
	extra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok","x":1}`)) }))
	notFound := httptest.NewServer(http.NotFoundHandler())
	starting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not available", 503) }))
	gone := httptest.NewServer(http.NotFoundHandler())
	goneURL := gone.URL
	gone.Close()
	for _, s := range []*httptest.Server{healthy, extra, notFound, starting} {
		defer s.Close()
	}

	ls404 := target("plaid", notFound)
	ls404.Source = wire.SourceLocalStack // a 404 from the gateway means LocalStack has no app emulator there
	got := Ready(context.Background(), nil,
		target("stripe", healthy), target("xero", extra), ls404, target("plaid", notFound), target("slack", starting),
		wire.Direct("resend", goneURL),
	)
	want := []State{Healthy, Unhealthy, NotIncluded, Unhealthy, Unhealthy, Unreachable}
	for i, w := range want {
		if got[i].State != w {
			t.Errorf("%s: state = %q, want %q (%+v)", got[i].Vendor, got[i].State, w, got[i])
		}
	}
	if !got[0].OK() || got[1].OK() {
		t.Error("OK should be true only for healthy")
	}
	if h := got[2].Hint; !strings.Contains(h, "PLAID_EMULATOR_URL") || !strings.Contains(h, "does not include plaid") {
		t.Errorf("not-included hint should say how to run it yourself: %q", h)
	}
	if h := got[5].Hint; !strings.Contains(h, "RESEND_EMULATOR_URL") {
		t.Errorf("unreachable hint should name the variable: %q", h)
	}
}

func TestResetRefusesNoTargets(t *testing.T) {
	if _, err := Reset(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "no default") {
		t.Fatalf("want a refusal, got %v", err)
	}
}

func TestResetWipesEveryTargetAndGoesOnPastAFailure(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	ok := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[name+" "+r.Method+" "+r.URL.Path]++
			mu.Unlock()
		}))
	}
	a, c := ok("a"), ok("c")
	defer a.Close()
	defer c.Close()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", 500) }))
	defer broken.Close()

	res, err := Reset(context.Background(), nil, target("stripe", a), target("plaid", broken), target("xero", c))
	if err == nil || !strings.Contains(err.Error(), "plaid") || strings.Contains(err.Error(), "stripe") {
		t.Fatalf("the error should name only the vendor that failed: %v", err)
	}
	if len(res) != 3 || res[0].Err != nil || res[1].Err == nil || res[2].Err != nil {
		t.Fatalf("unexpected results: %+v", res)
	}
	if hits["a POST /admin/reset"] != 1 || hits["c POST /admin/reset"] != 1 {
		t.Fatalf("both healthy targets must be reset, even after a failure: %v", hits)
	}
}

func TestAdvanceClockSendsAGoDuration(t *testing.T) {
	var body, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, path = string(b), r.URL.Path
	}))
	defer srv.Close()
	if err := AdvanceClock(context.Background(), nil, target("stripe", srv), 90*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if path != "/admin/time/advance" || body != `{"duration":"2160h0m0s"}` {
		t.Fatalf("path=%q body=%q", path, body)
	}
}

func TestRunStopsAtTheFirstFailureAndNamesTheStep(t *testing.T) {
	var ran []string
	var events []StepEvent
	boom := errors.New("boom")
	steps := []Step{
		{Name: "one", Do: func(context.Context) error { ran = append(ran, "one"); return nil }},
		{Name: "two", Do: func(context.Context) error { ran = append(ran, "two"); time.Sleep(2 * time.Millisecond); return boom }},
		{Name: "three", Do: func(context.Context) error { ran = append(ran, "three"); return nil }},
	}
	err := Run(context.Background(), steps, func(e StepEvent) { events = append(events, e) })
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), `"two"`) {
		t.Fatalf("want the step named and the cause kept: %v", err)
	}
	if strings.Join(ran, ",") != "one,two" || len(events) != 2 || events[1].Err == nil || events[1].Elapsed <= 0 {
		t.Fatalf("ran=%v events=%+v", ran, events)
	}
}

// The UI reads the hint from the API, so it has to be in the JSON.
func TestTheHintIsPartOfTheStatusJSON(t *testing.T) {
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	target := wire.Direct("plaid", notFound.URL)
	target.Source = wire.SourceLocalStack

	raw, err := json.Marshal(Ready(context.Background(), nil, target)[0])
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	json.Unmarshal(raw, &got)
	if got["state"] != "not-included" || !strings.Contains(got["hint"].(string), "PLAID_EMULATOR_URL") {
		t.Fatalf("the status JSON should carry the hint: %s", raw)
	}

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) }))
	defer healthy.Close()
	raw, _ = json.Marshal(Ready(context.Background(), nil, wire.Direct("stripe", healthy.URL))[0])
	if strings.Contains(string(raw), "hint") {
		t.Fatalf("a healthy app emulator needs no hint: %s", raw)
	}
}
