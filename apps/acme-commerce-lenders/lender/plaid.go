package lender

import (
	"context"
	"fmt"
	"net/http"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// PlaidSignals is what the lender learns from the merchant's bank connection.
//
// BalanceCents and Transactions are recorded but not used to price an offer. The
// Plaid app emulator's fixture is too thin to say anything about a merchant's
// cash, and a $110 balance would contradict the Stripe sales.
type PlaidSignals struct {
	Verified     bool   `json:"verified"` // the checking account exists on the connection
	Institution  string `json:"institution"`
	Mask         string `json:"mask"`
	BalanceCents int64  `json:"balance_cents"`
	Transactions int    `json:"transactions"`
}

func plaidBody(f world.Facts, extra map[string]any) map[string]any {
	m := map[string]any{"client_id": "acme-lender-client", "secret": "acme-lender-secret", "access_token": f.Plaid.AccessToken}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

type plaidAccount struct {
	AccountID string `json:"account_id"`
	Subtype   string `json:"subtype"`
	Mask      string `json:"mask"`
	Balances  struct {
		Current float64 `json:"current"`
	} `json:"balances"`
}

func (v Vendors) plaidAccounts(ctx context.Context, f world.Facts) ([]plaidAccount, error) {
	var out struct {
		Accounts []plaidAccount `json:"accounts"`
	}
	if err := v.Plaid.JSON(ctx, http.MethodPost, "/accounts/get", nil, plaidBody(f, nil), &out); err != nil {
		return nil, fmt.Errorf("list the merchant's bank accounts: %w", err)
	}
	return out.Accounts, nil
}

func (v Vendors) connectPlaid(ctx context.Context, f world.Facts) (Connection, error) {
	var item struct {
		Item struct {
			InstitutionID string `json:"institution_id"`
		} `json:"item"`
	}
	if err := v.Plaid.JSON(ctx, http.MethodPost, "/item/get", nil, plaidBody(f, nil), &item); err != nil {
		return Connection{}, fmt.Errorf("reach the merchant's bank connection: %w", err)
	}
	accounts, err := v.plaidAccounts(ctx, f)
	if err != nil {
		return Connection{}, err
	}
	for _, a := range accounts {
		if a.AccountID == f.Plaid.AccountID {
			return Connection{Vendor: "plaid", Detail: fmt.Sprintf("checking account ending %s at %s", a.Mask, item.Item.InstitutionID)}, nil
		}
	}
	return Connection{}, fmt.Errorf("the bank connection has no account %s", f.Plaid.AccountID)
}

func (v Vendors) readPlaid(ctx context.Context, f world.Facts) (PlaidSignals, error) {
	var s PlaidSignals
	accounts, err := v.plaidAccounts(ctx, f)
	if err != nil {
		return s, err
	}
	for _, a := range accounts {
		if a.AccountID == f.Plaid.AccountID {
			s.Verified, s.Mask = true, a.Mask
			s.BalanceCents = int64(a.Balances.Current*100 + 0.5)
		}
	}
	s.Institution = f.Plaid.Institution

	var txs struct {
		Total int `json:"total_transactions"`
	}
	if err := v.Plaid.JSON(ctx, http.MethodPost, "/transactions/get", nil, plaidBody(f, map[string]any{
		"start_date": "2000-01-01", "end_date": "2100-01-01",
	}), &txs); err != nil {
		return s, fmt.Errorf("count the merchant's bank transactions: %w", err)
	}
	s.Transactions = txs.Total
	return s, nil
}
