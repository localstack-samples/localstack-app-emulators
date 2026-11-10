package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// WorldAdmin is the Admin over the real app emulators.
type WorldAdmin struct {
	Targets world.Targets
	HC      *http.Client
	Store   core.Store
	Rec     *wire.Recorder
}

// Status checks every app emulator and whether the world is seeded.
func (a WorldAdmin) Status(ctx context.Context) (Status, error) {
	st := Status{Emulators: seed.Ready(ctx, a.HC, a.Targets.All()...)}
	healthy := true
	for _, e := range st.Emulators {
		healthy = healthy && e.OK()
	}
	facts, err := world.LoadFacts(ctx, a.Store)
	switch {
	case err == nil:
		st.Seeded, st.Facts = true, &facts
	case !errors.Is(err, core.ErrNotFound):
		return st, err
	}
	// Only a healthy app emulator can be asked what it holds. An unhealthy one is
	// already reported, and its saved world may still be there when it is back.
	if healthy && st.Seeded {
		st.Stale = world.Verify(ctx, a.HC, a.Targets, facts)
	}
	st.Ready = healthy && st.Seeded && len(st.Stale) == 0
	return st, nil
}

// Reset wipes the app emulators and the lender's state, seeds the world again,
// and forgets the recorded vendor calls from before.
func (a WorldAdmin) Reset(ctx context.Context) (world.Facts, error) {
	facts, err := world.Reset(ctx, a.HC, a.Targets, a.Store, nil)
	if err != nil {
		return facts, err
	}
	a.Rec.Clear()
	return facts, nil
}
