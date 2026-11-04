// Package api serves the demo's HTTP API: the application flow, the state of
// the world, a reset, and the stream of vendor calls the inspector shows.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/lender"
	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// Applications is the lender's application flow.
type Applications interface {
	Apply(ctx context.Context) (lender.Application, error)
	Get(ctx context.Context, id string) (lender.Application, error)
	Connect(ctx context.Context, id string) (lender.Application, error)
	Review(ctx context.Context, id string) (lender.Application, error)
	Offer(ctx context.Context, id string) (lender.Application, error)
}

// Admin reports on the world and puts it back at its start.
type Admin interface {
	Status(ctx context.Context) (Status, error)
	Reset(ctx context.Context) (world.Facts, error)
}

// Status is what the UI needs to know before it lets a presenter start.
type Status struct {
	Ready     bool          `json:"ready"` // every app emulator is healthy and the world is seeded
	Seeded    bool          `json:"seeded"`
	Emulators []seed.Status `json:"emulators"`
	Facts     *world.Facts  `json:"facts,omitempty"`
}

// New returns the API handler, to be mounted at /api/.
func New(apps Applications, admin Admin, rec *wire.Recorder) http.Handler {
	h := &handler{apps: apps, admin: admin, rec: rec}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", h.status)
	mux.HandleFunc("POST /api/reset", h.reset)
	mux.HandleFunc("POST /api/applications", h.apply)
	mux.HandleFunc("GET /api/applications/{id}", h.step(apps.Get, http.StatusOK))
	mux.HandleFunc("POST /api/applications/{id}/connect", h.step(apps.Connect, http.StatusOK))
	mux.HandleFunc("POST /api/applications/{id}/review", h.step(apps.Review, http.StatusOK))
	mux.HandleFunc("POST /api/applications/{id}/offer", h.step(apps.Offer, http.StatusOK))
	mux.HandleFunc("GET /api/events", h.events)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, errors.New("no such API route: "+r.Method+" "+r.URL.Path))
	})
	return mux
}

type handler struct {
	apps  Applications
	admin Admin
	rec   *wire.Recorder
	// A reset wipes everything the other calls read, so it waits for them and
	// they wait for it.
	mu sync.RWMutex
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	st, err := h.admin.Status(r.Context())
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *handler) reset(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	facts, err := h.admin.Reset(r.Context())
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (h *handler) apply(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	a, err := h.apps.Apply(r.Context())
	if err != nil {
		writeError(w, statusFor(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *handler) step(fn func(context.Context, string) (lender.Application, error), ok int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		defer h.mu.RUnlock()
		a, err := fn(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, statusFor(err), err)
			return
		}
		writeJSON(w, ok, a)
	}
}

// events returns the vendor calls recorded after the id in ?since, so a client
// can poll for just what is new.
func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	var since uint64
	if s := r.URL.Query().Get("since"); s != "" {
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("since must be an event id"))
			return
		}
		since = n
	}
	evs := h.rec.Since(since)
	last := since
	if len(evs) > 0 {
		last = evs[len(evs)-1].ID
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": evs, "last": last})
}

// statusFor maps an error to an HTTP status. A failure the lender could not
// have avoided is a vendor failure, so it is a bad gateway.
func statusFor(err error) int {
	var wrong *lender.WrongStateError
	switch {
	case errors.Is(err, lender.ErrNotFound):
		return http.StatusNotFound
	case errors.As(err, &wrong), errors.Is(err, lender.ErrNotSeeded):
		return http.StatusConflict
	}
	return http.StatusBadGateway
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
