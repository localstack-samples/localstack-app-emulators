package lender

import (
	"strings"
	"testing"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// signalsFromTheWorld is what the lender reads from the seeded merchant, worked
// out directly from the world's orders.
func signalsFromTheWorld() Signals {
	var s Signals
	for _, o := range world.Orders() {
		s.Stripe.Sales90Cents += o.Cents
		s.Stripe.Orders90++
		switch {
		case o.Day >= world.Days-30:
			s.Stripe.Sales30Cents += o.Cents
			s.Stripe.Orders30++
		case o.Day >= world.Days-60:
			s.Stripe.SalesPrev30Cents += o.Cents
		}
	}
	s.Stripe.Payouts, s.Stripe.PayoutCents = 12, 7504912
	s.Xero = XeroSignals{Invoices: 6, BilledCents: 3450000, OutstandingCents: 1615000, OverdueCents: 540000}
	s.Plaid = PlaidSignals{Verified: true, Mask: "0000", BalanceCents: 11000, Transactions: 2}
	return s
}

// These numbers are what the demo tells the audience. If a change moves them,
// that should be a decision and not an accident.
func TestTheSeededMerchantsOfferIsPinned(t *testing.T) {
	o := MakeOffer(signalsFromTheWorld())
	if !o.Approved {
		t.Fatalf("the seeded merchant should be approved: %s", o.Reason)
	}
	if o.AdvanceCents != 3000000 || o.FeeCents != 240000 || o.TotalRepayCents != 3240000 || o.GrowthPercent != 14.8 {
		t.Fatalf("the offer changed: advance %d, fee %d, repay %d, growth %v", o.AdvanceCents, o.FeeCents, o.TotalRepayCents, o.GrowthPercent)
	}
}

func TestTheAdvanceIsTheLast30DaysRoundedDownToFiveHundredDollars(t *testing.T) {
	for sales, want := range map[int64]int64{
		3099999: 3050000, // $30,999.99 rounds down to $30,500
		3050000: 3050000,
		49999:   0, // under $500 is too small
		50000:   50000,
	} {
		s := signalsFromTheWorld()
		s.Stripe.Sales30Cents = sales
		o := MakeOffer(s)
		if o.AdvanceCents != want {
			t.Errorf("sales %d: advance %d, want %d (%s)", sales, o.AdvanceCents, want, o.Reason)
		}
		if want > 0 && (o.FeeCents != want*8/100 || o.TotalRepayCents != want+o.FeeCents) {
			t.Errorf("sales %d: fee %d, repay %d", sales, o.FeeCents, o.TotalRepayCents)
		}
	}
}

func TestADeclineSaysWhy(t *testing.T) {
	cases := map[string]func(*Signals){
		"bank account could not be verified": func(s *Signals) { s.Plaid.Verified = false },
		"Only 12 orders in 90 days":          func(s *Signals) { s.Stripe.Orders90 = 12 },
		"No card sales in the last 30 days":  func(s *Signals) { s.Stripe.Sales30Cents = 0 },
		"too small to advance against":       func(s *Signals) { s.Stripe.Sales30Cents = 10000 },
	}
	for want, mutate := range cases {
		s := signalsFromTheWorld()
		mutate(&s)
		o := MakeOffer(s)
		if o.Approved || o.AdvanceCents != 0 || !strings.Contains(o.Reason, want) {
			t.Errorf("want a decline mentioning %q: %+v", want, o)
		}
		if len(o.Inputs) == 0 {
			t.Errorf("a decline still shows its inputs (%q)", want)
		}
	}
}

// The balance is shown, and it must not change the offer.
func TestThePlaidBalanceDoesNotChangeTheOffer(t *testing.T) {
	a := signalsFromTheWorld()
	b := signalsFromTheWorld()
	b.Plaid.BalanceCents = 99999999
	if MakeOffer(a).AdvanceCents != MakeOffer(b).AdvanceCents {
		t.Fatal("the Plaid balance must not move the advance")
	}
	for _, in := range MakeOffer(a).Inputs {
		if in.Label == "Checking balance" && in.Used {
			t.Fatal("the balance is shown but marked as not used")
		}
	}
}

func TestEveryInputNamesTheVendorCallItCameFrom(t *testing.T) {
	for _, in := range MakeOffer(signalsFromTheWorld()).Inputs {
		if in.Source == "" || in.Label == "" || in.Value == "" || !strings.HasPrefix(in.Call, in.Source+" ") {
			t.Errorf("an input must say where it came from: %+v", in)
		}
	}
}

func TestMoney(t *testing.T) {
	for cents, want := range map[int64]string{0: "$0.00", 5: "$0.05", 11000: "$110.00", 540000: "$5,400.00", 8108384: "$81,083.84", 123456789: "$1,234,567.89", -2500: "-$25.00"} {
		if got := money(cents); got != want {
			t.Errorf("money(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestGrowthIsZeroWhenThereIsNoEarlierPeriod(t *testing.T) {
	s := signalsFromTheWorld()
	s.Stripe.SalesPrev30Cents = 0
	if g := MakeOffer(s).GrowthPercent; g != 0 {
		t.Fatalf("growth = %v, want 0 with nothing to compare against", g)
	}
}
