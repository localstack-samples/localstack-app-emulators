package world

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// PlaidFacts describes the bank connection seeded into the Plaid app emulator.
//
// The Plaid app emulator gives every new Item the same small fixture: a few
// accounts, two transactions and no deposits. It cannot be given a merchant's
// own history yet, so Transactions and BalanceCents show how thin it is, and
// they do not match the Stripe payouts.
type PlaidFacts struct {
	Institution  string `json:"institution"`
	ItemID       string `json:"item_id"`
	AccessToken  string `json:"access_token"`
	AccountID    string `json:"account_id"` // the checking account
	Mask         string `json:"mask"`
	Transactions int    `json:"transactions"`
	BalanceCents int64  `json:"balance_cents"`
}

const plaidInstitution = "ins_109508"

// SeedPlaid connects a bank account for the merchant the way a Plaid Link
// session would, through the sandbox. The app emulator's clock is moved to the
// world's date first, so the fixture's transactions fall in the last few days
// of the history.
func SeedPlaid(ctx context.Context, hc *http.Client, t wire.Target) (PlaidFacts, error) {
	var facts PlaidFacts
	api := seed.API{HC: hc, Target: t}
	creds := map[string]any{"client_id": "acme-demo-client", "secret": "acme-demo-secret"}
	body := func(extra map[string]any) map[string]any {
		m := map[string]any{}
		for k, v := range creds {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	if err := seed.AdvanceClock(ctx, hc, t, Days*24*time.Hour); err != nil {
		return facts, fmt.Errorf("advance the clock: %w", err)
	}

	var pub struct {
		PublicToken string `json:"public_token"`
	}
	if err := api.JSON(ctx, http.MethodPost, "/sandbox/public_token/create", nil, body(map[string]any{
		"institution_id":   plaidInstitution,
		"initial_products": []string{"transactions", "auth"},
	}), &pub); err != nil {
		return facts, fmt.Errorf("create a sandbox Item: %w", err)
	}

	var ex struct {
		AccessToken string `json:"access_token"`
		ItemID      string `json:"item_id"`
	}
	if err := api.JSON(ctx, http.MethodPost, "/item/public_token/exchange", nil, body(map[string]any{"public_token": pub.PublicToken}), &ex); err != nil {
		return facts, fmt.Errorf("exchange the public token: %w", err)
	}
	facts.Institution, facts.AccessToken, facts.ItemID = plaidInstitution, ex.AccessToken, ex.ItemID

	var accts struct {
		Accounts []struct {
			AccountID string `json:"account_id"`
			Subtype   string `json:"subtype"`
			Mask      string `json:"mask"`
			Balances  struct {
				Current float64 `json:"current"`
			} `json:"balances"`
		} `json:"accounts"`
	}
	if err := api.JSON(ctx, http.MethodPost, "/accounts/get", nil, body(map[string]any{"access_token": ex.AccessToken}), &accts); err != nil {
		return facts, fmt.Errorf("list the accounts: %w", err)
	}
	found := false
	for _, a := range accts.Accounts {
		if a.Subtype == "checking" {
			facts.AccountID, facts.Mask = a.AccountID, a.Mask
			facts.BalanceCents = int64(a.Balances.Current*100 + 0.5)
			found = true
			break
		}
	}
	if !found {
		return facts, fmt.Errorf("the Item has no checking account among its %d accounts", len(accts.Accounts))
	}

	var txs struct {
		Total int `json:"total_transactions"`
	}
	if err := api.JSON(ctx, http.MethodPost, "/transactions/get", nil, body(map[string]any{
		"access_token": ex.AccessToken,
		"start_date":   "2000-01-01",
		"end_date":     "2100-01-01",
	}), &txs); err != nil {
		return facts, fmt.Errorf("count the transactions: %w", err)
	}
	facts.Transactions = txs.Total
	return facts, nil
}
