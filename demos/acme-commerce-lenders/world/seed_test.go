package world

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/core"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

type rig struct {
	stripe *fakeStripe
	plaid  *fake
	xero   *fake
	tg     Targets
}

func newRig(t *testing.T) *rig {
	r := &rig{stripe: newFakeStripe(t), plaid: newFakePlaid(t, plaidAccounts), xero: newFakeXero(t)}
	r.tg = Targets{
		Stripe: wire.Direct("stripe", r.stripe.srv.URL),
		Plaid:  wire.Direct("plaid", r.plaid.srv.URL),
		Xero:   wire.Direct("xero", r.xero.srv.URL),
	}
	return r
}

func TestSeedSeedsAllThreeAndSavesTheFacts(t *testing.T) {
	r := newRig(t)
	store := core.NewMemStore()
	var steps []string
	facts, err := Seed(context.Background(), nil, r.tg, store, func(e seed.StepEvent) { steps = append(steps, e.Step) })
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 || !strings.HasPrefix(steps[0], "stripe") || !strings.HasPrefix(steps[1], "plaid") || !strings.HasPrefix(steps[2], "xero") {
		t.Fatalf("steps should run in order and be reported: %v", steps)
	}
	if facts.Merchant != MerchantName || facts.Stripe.AccountID != "acct_merchant" || facts.Plaid.AccountID != "acc-chk" || facts.Xero.Invoices != 6 {
		t.Fatalf("unexpected facts: %+v", facts)
	}
	saved, err := LoadFacts(context.Background(), store)
	if err != nil || saved != facts {
		t.Fatalf("the saved facts should match: %+v, %v", saved, err)
	}
	if len(facts.AsOf) != len("2006-01-02") {
		t.Fatalf("AsOf should be a date: %q", facts.AsOf)
	}
	if got := time.Unix(facts.AsOfUnix, 0).UTC().Format("2006-01-02"); got != facts.AsOf {
		t.Fatalf("AsOfUnix (%s) and AsOf (%s) must name the same day", got, facts.AsOf)
	}
	if ahead := time.Until(time.Unix(facts.AsOfUnix, 0)); ahead < (Days-1)*24*time.Hour || ahead > (Days+1)*24*time.Hour {
		t.Fatalf("the history should end about %d days from now, got %s", Days, ahead)
	}
}

func TestSeedRefusesAWorldThatIsAlreadySeededAndTouchesNothing(t *testing.T) {
	r := newRig(t)
	store := core.NewMemStore()
	if _, err := Seed(context.Background(), nil, r.tg, store, nil); err != nil {
		t.Fatal(err)
	}
	before := len(r.xero.paths())
	if _, err := Seed(context.Background(), nil, r.tg, store, nil); !errors.Is(err, ErrAlreadySeeded) {
		t.Fatalf("want ErrAlreadySeeded, got %v", err)
	}
	if len(r.xero.paths()) != before {
		t.Fatal("a refused seed must not call the app emulators")
	}
}

func TestSeedChangesNothingWhenAnAppEmulatorIsNotReady(t *testing.T) {
	r := newRig(t)
	down := httptest.NewServer(nil)
	downURL := down.URL
	down.Close()
	r.tg.Plaid = wire.Direct("plaid", downURL)

	_, err := Seed(context.Background(), nil, r.tg, core.NewMemStore(), nil)
	if err == nil || !strings.Contains(err.Error(), "plaid") || !strings.Contains(err.Error(), "PLAID_EMULATOR_URL") {
		t.Fatalf("the error should name Plaid and say what to do: %v", err)
	}
	if len(r.stripe.intents) != 0 || len(r.xero.to("PUT", xeroPrefix+"/Contacts")) != 0 {
		t.Fatal("nothing may be seeded into the healthy app emulators when another one is down")
	}
}

func TestResetWipesEveryAppEmulatorClearsTheStoreThenSeedsAgain(t *testing.T) {
	r := newRig(t)
	store := core.NewMemStore()
	ctx := context.Background()
	if _, err := Seed(ctx, nil, r.tg, store, nil); err != nil {
		t.Fatal(err)
	}
	store.Put(ctx, "something/else", []byte("left over"))

	facts, err := Reset(ctx, nil, r.tg, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.stripe.resets != 1 || len(r.plaid.to("POST", "/admin/reset")) != 1 || len(r.xero.to("POST", "/admin/reset")) != 1 {
		t.Fatal("every app emulator must be wiped exactly once")
	}
	if _, err := store.Get(ctx, "something/else"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("the lender's own state must be cleared")
	}
	if got, err := LoadFacts(ctx, store); err != nil || got != facts {
		t.Fatalf("the world should be seeded again: %+v, %v", got, err)
	}
	// Two seeds in total: both the first and the post-reset run built the whole history.
	if len(r.stripe.intents) != 2*len(Orders()) {
		t.Fatalf("expected two full seeds, got %d payment intents", len(r.stripe.intents))
	}
}

func TestTargetsFromEnvMixesSelfRunAndLocalStack(t *testing.T) {
	env := map[string]string{"PLAID_EMULATOR_URL": "http://127.0.0.1:5101", "XERO_EMULATOR_URL": "http://127.0.0.1:5102"}
	tg := TargetsFromEnv(func(k string) string { return env[k] })
	if tg.Stripe.Source != wire.SourceLocalStack || tg.Plaid.Source != wire.SourceLocal || tg.Xero.Source != wire.SourceLocal {
		t.Fatalf("Stripe from LocalStack, Plaid and Xero self-run: %+v", tg)
	}
	if len(tg.All()) != 3 {
		t.Fatal("All should list three targets")
	}
}
