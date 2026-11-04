package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// Targets are the three app emulators the world is seeded into.
type Targets struct {
	Stripe, Plaid, Xero wire.Target
}

// TargetsFromEnv chooses each vendor's target with wire.TargetFromEnv: a vendor
// with <VENDOR>_EMULATOR_URL set is a self-run app emulator, and any other is
// served by LocalStack.
func TargetsFromEnv(getenv func(string) string) Targets {
	return Targets{
		Stripe: wire.TargetFromEnv(getenv, "stripe"),
		Plaid:  wire.TargetFromEnv(getenv, "plaid"),
		Xero:   wire.TargetFromEnv(getenv, "xero"),
	}
}

// All lists the targets in a fixed order.
func (t Targets) All() []wire.Target { return []wire.Target{t.Stripe, t.Plaid, t.Xero} }

// ErrAlreadySeeded means the store already holds a seeded world. Seeding twice
// would give the merchant a second history, so the caller resets first.
var ErrAlreadySeeded = errors.New("the world is already seeded; reset it first")

// CheckReady fails unless every app emulator is healthy, and says what to do
// about each one that is not.
func CheckReady(ctx context.Context, hc *http.Client, tg Targets) ([]seed.Status, error) {
	statuses := seed.Ready(ctx, hc, tg.All()...)
	var problems []string
	for _, s := range statuses {
		if !s.OK() {
			problems = append(problems, fmt.Sprintf("%s (%s) is %s: %s", s.Vendor, s.Source, s.State, s.Hint()))
		}
	}
	if len(problems) > 0 {
		return statuses, errors.New(strings.Join(problems, "\n"))
	}
	return statuses, nil
}

// LoadFacts reads the seeded world from the store.
func LoadFacts(ctx context.Context, store core.Store) (Facts, error) {
	var f Facts
	raw, err := store.Get(ctx, StoreKey)
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(raw, &f)
}

// Seed checks the app emulators, seeds each one, and saves what the lender
// learns about the merchant. It touches nothing if the world is already seeded
// or an app emulator is not ready. on, if not nil, hears about each step.
func Seed(ctx context.Context, hc *http.Client, tg Targets, store core.Store, on func(seed.StepEvent)) (Facts, error) {
	var facts Facts
	if _, err := store.Get(ctx, StoreKey); err == nil {
		return facts, ErrAlreadySeeded
	} else if !errors.Is(err, core.ErrNotFound) {
		return facts, err
	}
	if _, err := CheckReady(ctx, hc, tg); err != nil {
		return facts, fmt.Errorf("not ready to seed:\n%w", err)
	}

	asOf := time.Now().UTC().AddDate(0, 0, Days)
	facts.Merchant = MerchantName
	facts.AsOf = asOf.Format("2006-01-02")

	err := seed.Run(ctx, []seed.Step{
		{Name: "stripe: connected account, sales and payouts", Do: func(ctx context.Context) (err error) {
			facts.Stripe, err = SeedStripe(ctx, hc, tg.Stripe)
			return err
		}},
		{Name: "plaid: bank connection", Do: func(ctx context.Context) (err error) {
			facts.Plaid, err = SeedPlaid(ctx, hc, tg.Plaid)
			return err
		}},
		{Name: "xero: wholesale invoices", Do: func(ctx context.Context) (err error) {
			facts.Xero, err = SeedXero(ctx, hc, tg.Xero, asOf)
			return err
		}},
		{Name: "save what the lender knows", Do: func(ctx context.Context) error {
			raw, err := json.Marshal(facts)
			if err != nil {
				return err
			}
			return store.Put(ctx, StoreKey, raw)
		}},
	}, on)
	return facts, err
}

// Reset wipes every app emulator and the lender's own state, then seeds the
// world again, so the demo is back at its starting point.
func Reset(ctx context.Context, hc *http.Client, tg Targets, store core.Store, on func(seed.StepEvent)) (Facts, error) {
	if _, err := seed.Reset(ctx, hc, tg.All()...); err != nil {
		return Facts{}, err
	}
	if err := store.Clear(ctx); err != nil {
		return Facts{}, fmt.Errorf("clear the lender's state: %w", err)
	}
	return Seed(ctx, hc, tg, store, on)
}
