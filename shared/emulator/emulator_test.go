package emulator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitHealthyReturnsWhenOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := WaitHealthy(ctx, srv.URL, 10*time.Millisecond); err != nil {
		t.Fatalf("WaitHealthy: %v", err)
	}
}

func TestWaitHealthyRetriesUntilOK(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := WaitHealthy(ctx, srv.URL, 10*time.Millisecond); err != nil {
		t.Fatalf("WaitHealthy: %v", err)
	}
	if calls.Load() < 3 {
		t.Fatalf("expected at least 3 polls, got %d", calls.Load())
	}
}

// The contract is "exactly {"status":"ok"}": an extra field is not healthy.
func TestWaitHealthyRejectsExtraFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","extra":true}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := WaitHealthy(ctx, srv.URL, 10*time.Millisecond); err == nil {
		t.Fatal("expected a timeout for a non-exact health body")
	}
}

func TestWaitHealthyStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// Nothing listens on this port.
	if err := WaitHealthy(ctx, "http://127.0.0.1:1", 10*time.Millisecond); err == nil {
		t.Fatal("expected an error when nothing answers")
	}
}
