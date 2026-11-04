package lender

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/world"
)

// Funding is how an accepted advance reached the merchant. Each field is filled
// as its step finishes, so a funding that stopped halfway shows how far it got.
type Funding struct {
	AmountCents    int64  `json:"amount_cents"`
	PlaidAccountID string `json:"plaid_account_id"`          // the account the merchant verified with Plaid
	BankAccountID  string `json:"bank_account_id,omitempty"` // Stripe's id for that bank account
	BankLast4      string `json:"bank_last4,omitempty"`
	RoutingNumber  string `json:"routing_number,omitempty"`
	TransferID     string `json:"transfer_id,omitempty"` // the lender's transfer to the merchant's Stripe account
	PayoutID       string `json:"payout_id,omitempty"`   // the merchant's payout to its bank
	PayoutStatus   string `json:"payout_status,omitempty"`
}

// The account number is read from Plaid to register the bank account with
// Stripe, and it is never kept: Funding holds only the last four digits.

// bankAccount returns the routing and account numbers Plaid holds for the
// merchant's verified checking account.
func (v Vendors) bankAccount(ctx context.Context, f world.Facts) (routing, account string, err error) {
	var out struct {
		Numbers struct {
			ACH []struct {
				AccountID string `json:"account_id"`
				Account   string `json:"account"`
				Routing   string `json:"routing"`
			} `json:"ach"`
		} `json:"numbers"`
	}
	if err := v.Plaid.JSON(ctx, http.MethodPost, "/auth/get", nil, plaidBody(f, nil), &out); err != nil {
		return "", "", fmt.Errorf("read the merchant's bank account numbers from Plaid: %w", err)
	}
	for _, n := range out.Numbers.ACH {
		if n.AccountID == f.Plaid.AccountID {
			return n.Routing, n.Account, nil
		}
	}
	return "", "", fmt.Errorf("Plaid has no account numbers for %s", f.Plaid.AccountID)
}

// registerBankAccount gives the merchant's Stripe account the bank account the
// merchant verified with Plaid, as the place payouts go.
func (v Vendors) registerBankAccount(ctx context.Context, f world.Facts, fund *Funding) error {
	routing, account, err := v.bankAccount(ctx, f)
	if err != nil {
		return err
	}
	var out struct {
		ID            string `json:"id"`
		Last4         string `json:"last4"`
		RoutingNumber string `json:"routing_number"`
	}
	if err := v.Stripe.Form(ctx, http.MethodPost, "/v1/accounts/"+f.Stripe.AccountID+"/external_accounts", platformKey(), url.Values{
		"external_account[object]":         {"bank_account"},
		"external_account[country]":        {"US"},
		"external_account[currency]":       {"usd"},
		"external_account[routing_number]": {routing},
		"external_account[account_number]": {account},
		"default_for_currency":             {"true"},
	}, &out); err != nil {
		return fmt.Errorf("register the merchant's bank account with Stripe: %w", err)
	}
	fund.BankAccountID, fund.BankLast4, fund.RoutingNumber = out.ID, out.Last4, out.RoutingNumber
	return nil
}

// idempotent makes a Stripe call safe to repeat: the same key replays the first
// answer, so a retry after a lost response never moves money twice.
func idempotent(hdr map[string]string, key string) map[string]string {
	out := map[string]string{"Idempotency-Key": key}
	for k, v := range hdr {
		out[k] = v
	}
	return out
}

// transfer sends the advance from the lender to the merchant's Stripe account.
func (v Vendors) transfer(ctx context.Context, f world.Facts, appID string, fund *Funding) error {
	var out struct {
		ID string `json:"id"`
	}
	if err := v.Stripe.Form(ctx, http.MethodPost, "/v1/transfers", idempotent(platformKey(), appID+"-transfer"), url.Values{
		"amount":                {strconv.FormatInt(fund.AmountCents, 10)},
		"currency":              {"usd"},
		"destination":           {f.Stripe.AccountID},
		"description":           {"Working capital advance"},
		"metadata[application]": {appID},
	}, &out); err != nil {
		return fmt.Errorf("transfer the advance to the merchant's Stripe account: %w", err)
	}
	fund.TransferID = out.ID
	return nil
}

// payout pays the advance out of the merchant's Stripe balance to its bank.
func (v Vendors) payout(ctx context.Context, f world.Facts, appID string, fund *Funding) error {
	var out struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := v.Stripe.Form(ctx, http.MethodPost, "/v1/payouts", idempotent(merchantKey(f.Stripe.AccountID), appID+"-payout"), url.Values{
		"amount":                {strconv.FormatInt(fund.AmountCents, 10)},
		"currency":              {"usd"},
		"description":           {"Working capital advance"},
		"metadata[application]": {appID},
	}, &out); err != nil {
		return fmt.Errorf("pay the advance out to the merchant's bank: %w", err)
	}
	fund.PayoutID, fund.PayoutStatus = out.ID, out.Status
	return nil
}

// Fund pays an accepted advance to the merchant: it registers the Plaid-verified
// bank account with Stripe, transfers the advance, and pays it out.
//
// Each step is saved as soon as it finishes, and a step that is already done is
// not repeated, so a funding that failed halfway picks up where it stopped. The
// Stripe calls also carry idempotency keys, so repeating one cannot pay twice.
func (s *Service) Fund(ctx context.Context, id string) (Application, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.load(ctx, id)
	if err != nil {
		return a, err
	}
	if a.Status != Accepted {
		return a, &WrongStateError{Step: "fund", Have: a.Status, Want: Accepted}
	}
	f, err := s.facts(ctx)
	if err != nil {
		return a, err
	}
	if a.Funding == nil {
		a.Funding = &Funding{AmountCents: a.Offer.AdvanceCents, PlaidAccountID: f.Plaid.AccountID}
	}
	fund := a.Funding

	for _, st := range []struct {
		done bool
		run  func() error
	}{
		{fund.BankAccountID != "", func() error { return s.Vendors.registerBankAccount(ctx, f, fund) }},
		{fund.TransferID != "", func() error { return s.Vendors.transfer(ctx, f, a.ID, fund) }},
		{fund.PayoutID != "", func() error { return s.Vendors.payout(ctx, f, a.ID, fund) }},
	} {
		if st.done {
			continue
		}
		if err := st.run(); err != nil {
			_ = s.save(ctx, a) // keep what finished, so a retry resumes
			return a, err
		}
		if err := s.save(ctx, a); err != nil {
			return a, err
		}
	}
	a.Status = Funded
	return a, s.save(ctx, a)
}
