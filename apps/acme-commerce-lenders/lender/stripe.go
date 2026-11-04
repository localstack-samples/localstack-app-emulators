package lender

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// StripeSignals is what the lender learns from the merchant's Stripe account.
type StripeSignals struct {
	Sales30Cents     int64 `json:"sales_30_cents"`      // the last 30 days
	SalesPrev30Cents int64 `json:"sales_prev_30_cents"` // the 30 days before that
	Sales90Cents     int64 `json:"sales_90_cents"`
	Orders30         int   `json:"orders_30"`
	Orders90         int   `json:"orders_90"`
	Payouts          int   `json:"payouts"`
	PayoutCents      int64 `json:"payout_cents"`
}

const maxPages = 200

func platformKey() map[string]string {
	return map[string]string{"Authorization": "Bearer " + world.StripeKey}
}

func merchantKey(accountID string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + world.StripeKey, "Stripe-Account": accountID}
}

func (v Vendors) connectStripe(ctx context.Context, f world.Facts) (Connection, error) {
	var acct struct {
		ID string `json:"id"`
	}
	if err := v.Stripe.JSON(ctx, http.MethodGet, "/v1/accounts/"+f.Stripe.AccountID, platformKey(), nil, &acct); err != nil {
		return Connection{}, fmt.Errorf("reach the merchant's Stripe account: %w", err)
	}
	return Connection{Vendor: "stripe", Detail: "connected account " + acct.ID}, nil
}

// readStripe adds up the merchant's card sales in the last 30 days, the 30
// before, and the last 90. "Now" is the moment the world's history ends, not
// the wall clock, because seeding moves the app emulator's clock ahead.
func (v Vendors) readStripe(ctx context.Context, f world.Facts) (StripeSignals, error) {
	var s StripeSignals
	now := time.Unix(f.AsOfUnix, 0)
	t30 := now.Add(-30 * 24 * time.Hour).Unix()
	t60 := now.Add(-60 * 24 * time.Hour).Unix()
	t90 := now.Add(-90 * 24 * time.Hour).Unix()
	hdr := merchantKey(f.Stripe.AccountID)

	after := ""
	for page := 0; page < maxPages; page++ {
		q := url.Values{"limit": {"100"}}
		if after != "" {
			q.Set("starting_after", after)
		}
		var list struct {
			Data []struct {
				ID       string `json:"id"`
				Amount   int64  `json:"amount"`
				Created  int64  `json:"created"`
				Status   string `json:"status"`
				Refunded bool   `json:"refunded"`
			} `json:"data"`
			HasMore bool `json:"has_more"`
		}
		if err := v.Stripe.JSON(ctx, http.MethodGet, "/v1/charges?"+q.Encode(), hdr, nil, &list); err != nil {
			return s, fmt.Errorf("list the merchant's charges: %w", err)
		}
		for _, c := range list.Data {
			if c.Status != "succeeded" || c.Refunded || c.Created < t90 {
				continue
			}
			s.Sales90Cents += c.Amount
			s.Orders90++
			switch {
			case c.Created >= t30:
				s.Sales30Cents += c.Amount
				s.Orders30++
			case c.Created >= t60:
				s.SalesPrev30Cents += c.Amount
			}
		}
		if !list.HasMore || len(list.Data) == 0 {
			break
		}
		after = list.Data[len(list.Data)-1].ID
	}

	var payouts struct {
		Data []struct {
			Amount int64 `json:"amount"`
		} `json:"data"`
	}
	if err := v.Stripe.JSON(ctx, http.MethodGet, "/v1/payouts?limit=100", hdr, nil, &payouts); err != nil {
		return s, fmt.Errorf("list the merchant's payouts: %w", err)
	}
	for _, p := range payouts.Data {
		s.Payouts++
		s.PayoutCents += p.Amount
	}
	return s, nil
}
