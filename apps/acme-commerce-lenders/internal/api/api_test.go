package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/internal/fakevendors"
	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/lender"
	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

type fakeApps struct {
	err   error
	calls []string
}

func (f *fakeApps) rec(name, id string) (lender.Application, error) {
	f.calls = append(f.calls, name+" "+id)
	return lender.Application{ID: id, Status: lender.Applied, Merchant: "M"}, f.err
}
func (f *fakeApps) Apply(context.Context) (lender.Application, error) {
	return f.rec("apply", "app-0001")
}
func (f *fakeApps) Get(_ context.Context, id string) (lender.Application, error) {
	return f.rec("get", id)
}
func (f *fakeApps) Connect(_ context.Context, id string) (lender.Application, error) {
	return f.rec("connect", id)
}
func (f *fakeApps) Review(_ context.Context, id string) (lender.Application, error) {
	return f.rec("review", id)
}
func (f *fakeApps) Offer(_ context.Context, id string) (lender.Application, error) {
	return f.rec("offer", id)
}
func (f *fakeApps) Accept(_ context.Context, id string) (lender.Application, error) {
	return f.rec("accept", id)
}
func (f *fakeApps) Fund(_ context.Context, id string) (lender.Application, error) {
	return f.rec("fund", id)
}
func (f *fakeApps) Settle(_ context.Context, id string) (lender.Application, error) {
	return f.rec("settle", id)
}

type fakeAdmin struct {
	resetErr error
	resets   int
	inReset  chan struct{}
	release  chan struct{}
}

func (f *fakeAdmin) Status(context.Context) (Status, error) {
	return Status{Ready: true, Seeded: true}, nil
}
func (f *fakeAdmin) Reset(context.Context) (world.Facts, error) {
	f.resets++
	if f.inReset != nil {
		close(f.inReset)
		<-f.release
	}
	return world.Facts{Merchant: "M"}, f.resetErr
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestRoutesCallTheLenderAndReturnJSON(t *testing.T) {
	apps := &fakeApps{}
	h := New(apps, &fakeAdmin{}, wire.NewRecorder(10))
	for _, tc := range []struct {
		method, path string
		status       int
		call         string
	}{
		{"POST", "/api/applications", 201, "apply app-0001"},
		{"GET", "/api/applications/app-0007", 200, "get app-0007"},
		{"POST", "/api/applications/app-0007/connect", 200, "connect app-0007"},
		{"POST", "/api/applications/app-0007/review", 200, "review app-0007"},
		{"POST", "/api/applications/app-0007/offer", 200, "offer app-0007"},
		{"POST", "/api/applications/app-0007/accept", 200, "accept app-0007"},
		{"POST", "/api/applications/app-0007/fund", 200, "fund app-0007"},
		{"POST", "/api/applications/app-0007/settle", 200, "settle app-0007"},
	} {
		rec := do(h, tc.method, tc.path)
		if rec.Code != tc.status || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("%s %s: %d %q", tc.method, tc.path, rec.Code, rec.Header().Get("Content-Type"))
		}
		if got := apps.calls[len(apps.calls)-1]; got != tc.call {
			t.Errorf("%s %s called %q, want %q", tc.method, tc.path, got, tc.call)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s %s should not be cached", tc.method, tc.path)
		}
	}
}

func TestErrorsMapToTheRightStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{lender.ErrNotFound, 404},
		{&lender.WrongStateError{Step: "offer", Have: lender.Applied, Want: lender.Reviewed}, 409},
		{lender.ErrNotSeeded, 409},
		{lender.ErrDeclined, 409},
		{errors.New("stripe GET /v1/charges: HTTP 500"), 502},
	} {
		rec := do(New(&fakeApps{err: tc.err}, &fakeAdmin{}, wire.NewRecorder(1)), "POST", "/api/applications/app-1/connect")
		var body map[string]string
		json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != tc.want || body["error"] != tc.err.Error() {
			t.Errorf("%v: status %d body %v, want %d with the message", tc.err, rec.Code, body, tc.want)
		}
	}
}

func TestUnknownRoutesAndMethodsAreRefused(t *testing.T) {
	h := New(&fakeApps{}, &fakeAdmin{}, wire.NewRecorder(1))
	if rec := do(h, "GET", "/api/nothing"); rec.Code != 404 || !strings.Contains(rec.Body.String(), "no such API route") {
		t.Errorf("unknown route: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "DELETE", "/api/applications/app-1"); rec.Code != 405 && rec.Code != 404 {
		t.Errorf("an unsupported method should be refused, got %d", rec.Code)
	}
}

func TestEventsReturnOnlyWhatIsNew(t *testing.T) {
	rec := wire.NewRecorder(10)
	for _, p := range []string{"/a", "/b", "/c"} {
		rec.Record(wire.Event{Vendor: "stripe", Path: p})
	}
	h := New(&fakeApps{}, &fakeAdmin{}, rec)

	var all struct {
		Events []wire.Event `json:"events"`
		Last   uint64       `json:"last"`
	}
	json.Unmarshal(do(h, "GET", "/api/events").Body.Bytes(), &all)
	if len(all.Events) != 3 || all.Last != 3 {
		t.Fatalf("all events: %+v", all)
	}
	var fresh struct {
		Events []wire.Event `json:"events"`
		Last   uint64       `json:"last"`
	}
	json.Unmarshal(do(h, "GET", "/api/events?since=2").Body.Bytes(), &fresh)
	if len(fresh.Events) != 1 || fresh.Events[0].Path != "/c" || fresh.Last != 3 {
		t.Fatalf("since=2: %+v", fresh)
	}
	json.Unmarshal(do(h, "GET", "/api/events?since=3").Body.Bytes(), &fresh)
	if len(fresh.Events) != 0 || fresh.Last != 3 {
		t.Fatalf("nothing new should keep the cursor: %+v", fresh)
	}
	if rec := do(h, "GET", "/api/events?since=abc"); rec.Code != 400 {
		t.Fatalf("a bad cursor is a 400, got %d", rec.Code)
	}
}

func TestEventsReportWhenTheRecordingWasCleared(t *testing.T) {
	rec := wire.NewRecorder(10)
	h := New(&fakeApps{}, &fakeAdmin{}, rec)
	generation := func() uint64 {
		var body struct {
			Generation uint64 `json:"generation"`
		}
		json.Unmarshal(do(h, "GET", "/api/events").Body.Bytes(), &body)
		return body.Generation
	}
	before := generation()
	rec.Record(wire.Event{Path: "/a"})
	if generation() != before {
		t.Fatal("recording a call must not change the generation")
	}
	rec.Clear()
	if generation() == before {
		t.Fatal("clearing the recording must change the generation")
	}
}

func TestStatusAndReset(t *testing.T) {
	admin := &fakeAdmin{}
	h := New(&fakeApps{}, admin, wire.NewRecorder(1))
	if rec := do(h, "GET", "/api/status"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ready":true`) {
		t.Errorf("status: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "POST", "/api/reset"); rec.Code != 200 || admin.resets != 1 {
		t.Errorf("reset: %d resets=%d", rec.Code, admin.resets)
	}
	admin.resetErr = errors.New("reset failed for plaid")
	if rec := do(h, "POST", "/api/reset"); rec.Code != 502 || !strings.Contains(rec.Body.String(), "plaid") {
		t.Errorf("a failed reset should say which vendor: %d %s", rec.Code, rec.Body.String())
	}
}

// A reset wipes what other calls read, so calls wait for it instead of reading half a world.
func TestACallWaitsWhileAResetIsRunning(t *testing.T) {
	admin := &fakeAdmin{inReset: make(chan struct{}), release: make(chan struct{})}
	apps := &fakeApps{}
	h := New(apps, admin, wire.NewRecorder(1))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); do(h, "POST", "/api/reset") }()
	<-admin.inReset // the reset is now running

	var done atomic.Bool
	wg.Add(1)
	go func() { defer wg.Done(); do(h, "POST", "/api/applications"); done.Store(true) }()
	time.Sleep(50 * time.Millisecond)
	if done.Load() {
		t.Fatal("an application call must wait for the reset to finish")
	}
	close(admin.release)
	wg.Wait()
	if !done.Load() || len(apps.calls) != 1 {
		t.Fatalf("the call should run once the reset is done: %v", apps.calls)
	}
}

func TestWorldAdminReportsReadinessAndResetsTheRecorder(t *testing.T) {
	var resets atomic.Int32
	srv := fakevendors.New(t, &resets)
	tg := world.Targets{Stripe: wire.Direct("stripe", srv.URL), Plaid: wire.Direct("plaid", srv.URL), Xero: wire.Direct("xero", srv.URL)}
	rec := wire.NewRecorder(10)
	rec.Record(wire.Event{Vendor: "stripe", Path: "/before"})
	admin := WorldAdmin{Targets: tg, Store: core.NewMemStore(), Rec: rec}
	ctx := context.Background()

	st, err := admin.Status(ctx)
	if err != nil || st.Seeded || st.Ready || len(st.Emulators) != 3 {
		t.Fatalf("an unseeded world is not ready even with healthy app emulators: %+v, %v", st, err)
	}
	facts, err := admin.Reset(ctx)
	if err != nil || facts.Merchant != world.MerchantName {
		t.Fatalf("reset: %+v, %v", facts, err)
	}
	if len(rec.Events()) != 0 {
		t.Fatal("a reset forgets the vendor calls from before")
	}
	if resets.Load() != 3 {
		t.Fatalf("every app emulator is wiped, got %d", resets.Load())
	}
	if st, err = admin.Status(ctx); err != nil || !st.Seeded || !st.Ready || st.Facts == nil || st.Facts.Merchant != world.MerchantName {
		t.Fatalf("after a reset the world is ready: %+v, %v", st, err)
	}
}

func TestTargetsReportAndChangeWhereEachVendorIsCalled(t *testing.T) {
	sw := wire.NewSwitch(
		wire.Lane{Vendor: "stripe", Name: "Stripe", Emulator: wire.LocalStack("stripe"), ServiceURL: "https://api.stripe.com"},
	)
	h := NewWithTargets(&fakeApps{}, &fakeAdmin{}, wire.NewRecorder(1), sw)

	get := func() wire.State {
		rec := do(h, "GET", "/api/targets")
		var st wire.State
		if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || rec.Code != 200 {
			t.Fatalf("GET /api/targets: %d %s", rec.Code, rec.Body)
		}
		return st
	}
	put := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/targets", strings.NewReader(body)))
		return rec
	}

	if st := get(); st.Mode != wire.ModeDemo || st.Style != wire.StyleDryRun || len(st.Lanes) != 1 {
		t.Fatalf("start: %+v", st)
	}
	if rec := put(`{"urls":{"stripe":"https://api.stripe.com"}}`); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	if st := get(); st.Mode != wire.ModeProduction || !st.Lanes[0].Service {
		t.Fatalf("after PUT: %+v", st)
	}
	if rec := put(`{"urls":{"stripe":"stripe"}}`); rec.Code != 200 || get().Lanes[0].Error == "" {
		t.Fatalf("an invalid URL is reported in the state: %d %+v", rec.Code, get())
	}
	for _, body := range []string{`{"style":"yolo"}`, `{"urls":{"nope":"http://x"}}`, `not json`} {
		if rec := put(body); rec.Code != 400 {
			t.Errorf("PUT %s: %d, want 400", body, rec.Code)
		}
	}
}
