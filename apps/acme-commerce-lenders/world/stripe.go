package world

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// StripeKey is the lender's own Stripe secret key. The Stripe app emulator
// rejects a /v1 call that does not carry one, as Stripe does.
const StripeKey = "sk_test_acme_lender_platform"

// StripeFacts describes what was seeded into the Stripe app emulator.
type StripeFacts struct {
	AccountID   string `json:"account_id"`   // the merchant's connected account
	Orders      int    `json:"orders"`       // card sales
	GrossCents  int64  `json:"gross_cents"`  // their total
	Payouts     int    `json:"payouts"`      // weekly payouts
	PayoutCents int64  `json:"payout_cents"` // their total
}

// SeedStripe gives the merchant a connected account with Days days of card
// sales and a payout for each full week.
//
// It builds the history by walking the app emulator's clock forward one day at
// a time, because Stripe does not let a caller set when something was created.
// When it returns, the app emulator's clock is Days days ahead of real time
// and the history ends today. A reset puts the clock back.
//
// The Stripe app emulator does not add a connected account's sales to its
// balance, so before each payout the account is funded with exactly that
// week's sales through the app emulator's admin API. Each payout then leaves
// the balance at zero, as an automatic payout would.
func SeedStripe(ctx context.Context, hc *http.Client, t wire.Target) (StripeFacts, error) {
	api := seed.API{HC: hc, Target: t}
	var facts StripeFacts

	var account struct {
		ID string `json:"id"`
	}
	platform := map[string]string{"Authorization": "Bearer " + StripeKey}
	if err := api.Form(ctx, http.MethodPost, "/v1/accounts", platform, url.Values{
		"type":                                   {"custom"},
		"country":                                {"US"},
		"business_type":                          {"company"},
		"company[name]":                          {MerchantName},
		"capabilities[card_payments][requested]": {"true"},
		"capabilities[transfers][requested]":     {"true"},
	}, &account); err != nil {
		return facts, fmt.Errorf("create the merchant's connected account: %w", err)
	}
	facts.AccountID = account.ID
	// The merchant's own sales are made on its connected account.
	acct := map[string]string{"Authorization": "Bearer " + StripeKey, "Stripe-Account": account.ID}

	// A payout needs somewhere to go. The funding step replaces this with the
	// bank account the merchant verified with Plaid. btok_us_verified is one of
	// Stripe's own test tokens. The Stripe app emulator refuses a token it does
	// not know, so a made-up one will not do.
	if err := api.Form(ctx, http.MethodPost, "/v1/accounts/"+account.ID+"/external_accounts", platform,
		url.Values{"external_account": {"btok_us_verified"}}, nil); err != nil {
		return facts, fmt.Errorf("add the merchant's bank account: %w", err)
	}

	orders := Orders()
	next := 0
	var week int64
	for day := 0; day < Days; day++ {
		for ; next < len(orders) && orders[next].Day == day; next++ {
			o := orders[next]
			if err := api.Form(ctx, http.MethodPost, "/v1/payment_intents", acct, url.Values{
				"amount":                             {strconv.FormatInt(o.Cents, 10)},
				"currency":                           {"usd"},
				"payment_method":                     {"pm_card_visa"},
				"confirm":                            {"true"},
				"description":                        {"Online order"},
				"metadata[world_order]":              {strconv.Itoa(next)},
				"automatic_payment_methods[enabled]": {"true"},
				"automatic_payment_methods[allow_redirects]": {"never"},
			}, nil); err != nil {
				return facts, fmt.Errorf("day %d, order %d: %w", day, next, err)
			}
			facts.Orders++
			facts.GrossCents += o.Cents
			week += o.Cents
		}

		if (day+1)%7 == 0 && week > 0 {
			if err := api.JSON(ctx, http.MethodPost, "/admin/accounts/"+account.ID+"/fund", nil,
				map[string]any{"amount": week, "currency": "usd"}, nil); err != nil {
				return facts, fmt.Errorf("day %d: fund the account for its payout: %w", day, err)
			}
			if err := api.Form(ctx, http.MethodPost, "/v1/payouts", acct, url.Values{
				"amount":   {strconv.FormatInt(week, 10)},
				"currency": {"usd"},
			}, nil); err != nil {
				return facts, fmt.Errorf("day %d: payout: %w", day, err)
			}
			facts.Payouts++
			facts.PayoutCents += week
			week = 0
		}

		if err := seed.AdvanceClock(ctx, hc, t, 24*time.Hour); err != nil {
			return facts, fmt.Errorf("day %d: advance the clock: %w", day, err)
		}
	}
	return facts, nil
}
