package lender

import (
	"fmt"
	"math"
)

// Signals is everything the lender learned about a merchant.
type Signals struct {
	Stripe StripeSignals `json:"stripe"`
	Plaid  PlaidSignals  `json:"plaid"`
	Xero   XeroSignals   `json:"xero"`
}

// Input is one fact an offer rests on, with the vendor call it came from, so a
// presenter can show where every number comes from.
type Input struct {
	Source string `json:"source"` // stripe, plaid or xero
	Label  string `json:"label"`
	Value  string `json:"value"`
	Call   string `json:"call"` // the vendor call that produced it
	Used   bool   `json:"used"` // whether it changed the offer
}

// Offer is what the lender puts to the merchant.
type Offer struct {
	Approved        bool    `json:"approved"`
	Reason          string  `json:"reason"` // why it was declined, or the one-line basis for approving
	AdvanceCents    int64   `json:"advance_cents"`
	FeePercent      float64 `json:"fee_percent"`
	FeeCents        int64   `json:"fee_cents"`
	TotalRepayCents int64   `json:"total_repay_cents"`
	GrowthPercent   float64 `json:"growth_percent"` // sales in the last 30 days against the 30 before
	Inputs          []Input `json:"inputs"`
}

// The terms. They are fictional numbers for a demo.
const (
	feePercent     = 8.0
	advanceStep    = 50000 // advances are rounded down to $500
	minOrders90    = 30    // fewer sales than this and there is too little history
	advanceCeiling = 1.0   // the advance is this multiple of the last 30 days' sales
)

// MakeOffer prices working capital from the merchant's signals. The advance is
// the last 30 days of card sales, rounded down to $500, and the fee is a flat 8%.
//
// The merchant's Plaid balance is shown but does not change the offer: the Plaid
// app emulator's fixture is too thin to say anything about the merchant's cash.
func MakeOffer(s Signals) Offer {
	o := Offer{FeePercent: feePercent}
	if s.Stripe.SalesPrev30Cents > 0 {
		o.GrowthPercent = round1(float64(s.Stripe.Sales30Cents-s.Stripe.SalesPrev30Cents) / float64(s.Stripe.SalesPrev30Cents) * 100)
	}
	o.Inputs = []Input{
		{"stripe", "Card sales, last 30 days", money(s.Stripe.Sales30Cents), "stripe GET /v1/charges", true},
		{"stripe", "Card sales, the 30 days before", money(s.Stripe.SalesPrev30Cents), "stripe GET /v1/charges", false},
		{"stripe", "Orders, last 90 days", fmt.Sprint(s.Stripe.Orders90), "stripe GET /v1/charges", true},
		{"stripe", "Sales growth", fmt.Sprintf("%+.1f%%", o.GrowthPercent), "stripe GET /v1/charges", false},
		{"stripe", "Paid out so far", money(s.Stripe.PayoutCents), "stripe GET /v1/payouts", false},
		{"xero", "Outstanding receivables", money(s.Xero.OutstandingCents), "xero GET /api.xro/2.0/Invoices", false},
		{"xero", "Overdue receivables", money(s.Xero.OverdueCents), "xero GET /api.xro/2.0/Invoices", false},
		{"plaid", "Checking account verified", yesNo(s.Plaid.Verified), "plaid POST /accounts/get", true},
		{"plaid", "Checking balance", money(s.Plaid.BalanceCents), "plaid POST /accounts/get", false},
	}

	switch {
	case !s.Plaid.Verified:
		o.Reason = "The merchant's bank account could not be verified."
	case s.Stripe.Orders90 < minOrders90:
		o.Reason = fmt.Sprintf("Only %d orders in 90 days; the lender needs at least %d.", s.Stripe.Orders90, minOrders90)
	case s.Stripe.Sales30Cents <= 0:
		o.Reason = "No card sales in the last 30 days."
	}
	if o.Reason != "" {
		return o
	}

	o.AdvanceCents = int64(math.Floor(float64(s.Stripe.Sales30Cents)*advanceCeiling/advanceStep)) * advanceStep
	if o.AdvanceCents == 0 {
		o.Reason = "The last 30 days of sales are too small to advance against."
		return o
	}
	o.Approved = true
	o.FeeCents = int64(math.Round(float64(o.AdvanceCents) * feePercent / 100))
	o.TotalRepayCents = o.AdvanceCents + o.FeeCents
	o.Reason = fmt.Sprintf("An advance of the last 30 days of card sales (%s), rounded down to $500, for a flat %.0f%% fee.", money(s.Stripe.Sales30Cents), feePercent)
	return o
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// money formats cents as dollars with thousands separators.
func money(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	s := fmt.Sprintf("%d.%02d", c/100, c%100)
	for i := len(s) - 6; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return sign + "$" + s
}
