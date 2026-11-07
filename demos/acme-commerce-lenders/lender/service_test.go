package lender

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

func newService(t *testing.T, seeded bool) (*Service, *fakeVendors, *wire.Recorder, core.Store) {
	t.Helper()
	f := newFakeVendors(t)
	rec := wire.NewRecorder(2000)
	store := core.NewMemStore()
	if seeded {
		raw, _ := json.Marshal(facts())
		store.Put(context.Background(), world.StoreKey, raw)
	}
	return &Service{Vendors: f.vendors(rec), Store: store}, f, rec, store
}

func TestAnApplicationGoesFromApplyToOffer(t *testing.T) {
	ctx := context.Background()
	svc, _, rec, _ := newService(t, true)

	a, err := svc.Apply(ctx)
	if err != nil || a.ID != "app-0001" || a.Status != Applied || a.Merchant != world.MerchantName {
		t.Fatalf("apply: %+v, %v", a, err)
	}
	if a, err = svc.Connect(ctx, a.ID); err != nil || a.Status != Connected || len(a.Connections) != 3 {
		t.Fatalf("connect: %+v, %v", a, err)
	}
	if a, err = svc.Review(ctx, a.ID); err != nil || a.Status != Reviewed || a.Signals == nil || a.Signals.Stripe.Orders90 != 941 {
		t.Fatalf("review: %+v, %v", a, err)
	}
	if a, err = svc.Offer(ctx, a.ID); err != nil || a.Status != Offered || a.Offer == nil {
		t.Fatalf("offer: %+v, %v", a, err)
	}
	if !a.Offer.Approved || a.Offer.AdvanceCents != 3000000 {
		t.Fatalf("the seeded merchant gets the pinned $30,000 advance: %+v", a.Offer)
	}
	if len(rec.Events()) < 10 {
		t.Fatalf("every vendor call should be recorded, got %d", len(rec.Events()))
	}
}

func TestStepsMustHappenInOrder(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newService(t, true)
	a, _ := svc.Apply(ctx)

	for name, run := range map[string]func() error{
		"review before connect": func() error { _, err := svc.Review(ctx, a.ID); return err },
		"offer before review":   func() error { _, err := svc.Offer(ctx, a.ID); return err },
	} {
		var wrong *WrongStateError
		if err := run(); !errors.As(err, &wrong) || wrong.Have != Applied {
			t.Errorf("%s: want a WrongStateError, got %v", name, err)
		}
	}
	svc.Connect(ctx, a.ID)
	var wrong *WrongStateError
	if _, err := svc.Connect(ctx, a.ID); !errors.As(err, &wrong) || wrong.Have != Connected {
		t.Fatalf("connecting twice should be refused: %v", err)
	}
}

func TestApplyNeedsASeededWorld(t *testing.T) {
	svc, _, _, _ := newService(t, false)
	if _, err := svc.Apply(context.Background()); !errors.Is(err, ErrNotSeeded) {
		t.Fatalf("want ErrNotSeeded, got %v", err)
	}
}

func TestAFailedStepSavesNothingAndCanBeRetried(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a, _ := svc.Apply(ctx)

	f.noChecking = true // Plaid will not find the account
	if _, err := svc.Connect(ctx, a.ID); err == nil {
		t.Fatal("connect should fail when the bank account is missing")
	}
	if got, _ := svc.Get(ctx, a.ID); got.Status != Applied || len(got.Connections) != 0 {
		t.Fatalf("a failed step must leave the application alone: %+v", got)
	}
	f.noChecking = false
	if got, err := svc.Connect(ctx, a.ID); err != nil || got.Status != Connected {
		t.Fatalf("the step should work on retry: %+v, %v", got, err)
	}
}

func TestApplicationsPersistAcrossServices(t *testing.T) {
	ctx := context.Background()
	svc, f, rec, store := newService(t, true)
	a, _ := svc.Apply(ctx)
	b, _ := svc.Apply(ctx)
	if a.ID == b.ID || b.ID != "app-0002" {
		t.Fatalf("each application gets its own id: %s, %s", a.ID, b.ID)
	}
	other := &Service{Vendors: f.vendors(rec), Store: store} // the server, after the CLI wrote
	if got, err := other.Get(ctx, a.ID); err != nil || got.ID != a.ID {
		t.Fatalf("a second service should see the application: %+v, %v", got, err)
	}
	if _, err := other.Get(ctx, "app-9999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
