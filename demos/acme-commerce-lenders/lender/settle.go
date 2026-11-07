package lender

import (
	"context"
	"fmt"
	"net/http"

	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
)

// The advance is a loan to the merchant, so it is booked as money received
// against a liability account, which debits the bank and credits the loan. The
// bank account is the Xero app emulator's default one.
const (
	advanceAccountCode = "810"
	advanceAccountName = "Working capital advance"
	bankAccountCode    = "090"
	lenderName         = "ACME Commerce Lenders"
)

// Settlement is how the advance appears in the merchant's Xero books.
type Settlement struct {
	AmountCents       int64  `json:"amount_cents"`
	BankTransactionID string `json:"bank_transaction_id"`
	BankAccountCode   string `json:"bank_account_code"`
	LiabilityCode     string `json:"liability_code"`
	LiabilityName     string `json:"liability_name"`
	Reference         string `json:"reference"` // the application's id
	Date              string `json:"date"`
}

// ensureAdvanceAccount makes sure the merchant's chart has the loan account,
// creating it only if it is missing, so settling twice never fails on a
// duplicate account code.
func (v Vendors) ensureAdvanceAccount(ctx context.Context, f world.Facts) error {
	var accounts struct {
		Accounts []struct {
			Code string `json:"Code"`
		} `json:"Accounts"`
	}
	if err := v.Xero.JSON(ctx, http.MethodGet, xeroBase+"/Accounts", xeroHeaders(f), nil, &accounts); err != nil {
		return fmt.Errorf("read the merchant's chart of accounts: %w", err)
	}
	for _, a := range accounts.Accounts {
		if a.Code == advanceAccountCode {
			return nil
		}
	}
	err := v.Xero.JSON(ctx, http.MethodPut, xeroBase+"/Accounts", xeroHeaders(f), map[string]string{
		"Code": advanceAccountCode, "Name": advanceAccountName, "Type": "CURRLIAB",
	}, nil)
	if err != nil {
		return fmt.Errorf("add the %q account to the merchant's chart: %w", advanceAccountName, err)
	}
	return nil
}

// ensureLender makes sure the lender is a contact in the merchant's books and
// returns its id.
func (v Vendors) ensureLender(ctx context.Context, f world.Facts) (string, error) {
	var contacts struct {
		Contacts []struct {
			ContactID string `json:"ContactID"`
			Name      string `json:"Name"`
		} `json:"Contacts"`
	}
	if err := v.Xero.JSON(ctx, http.MethodGet, xeroBase+"/Contacts", xeroHeaders(f), nil, &contacts); err != nil {
		return "", fmt.Errorf("read the merchant's contacts: %w", err)
	}
	for _, c := range contacts.Contacts {
		if c.Name == lenderName {
			return c.ContactID, nil
		}
	}
	var created struct {
		Contacts []struct {
			ContactID string `json:"ContactID"`
		} `json:"Contacts"`
	}
	if err := v.Xero.JSON(ctx, http.MethodPut, xeroBase+"/Contacts", xeroHeaders(f),
		map[string]any{"Contacts": []map[string]string{{"Name": lenderName}}}, &created); err != nil {
		return "", fmt.Errorf("add the lender to the merchant's contacts: %w", err)
	}
	if len(created.Contacts) == 0 {
		return "", fmt.Errorf("Xero did not return the lender it created")
	}
	return created.Contacts[0].ContactID, nil
}

// findAdvance looks for a bank transaction this application already recorded,
// by its reference, so a retry after a lost answer does not record it twice.
func (v Vendors) findAdvance(ctx context.Context, f world.Facts, ref string) (string, bool, error) {
	var txs struct {
		BankTransactions []struct {
			BankTransactionID string `json:"BankTransactionID"`
			Reference         string `json:"Reference"`
		} `json:"BankTransactions"`
	}
	if err := v.Xero.JSON(ctx, http.MethodGet, xeroBase+"/BankTransactions", xeroHeaders(f), nil, &txs); err != nil {
		return "", false, fmt.Errorf("read the merchant's bank transactions: %w", err)
	}
	for _, t := range txs.BankTransactions {
		if t.Reference == ref {
			return t.BankTransactionID, true, nil
		}
	}
	return "", false, nil
}

// recordAdvance books the advance in the merchant's Xero as money received, and
// returns where it was booked.
func (v Vendors) recordAdvance(ctx context.Context, f world.Facts, appID string, fund Funding) (Settlement, error) {
	s := Settlement{
		AmountCents: fund.AmountCents, BankAccountCode: bankAccountCode,
		LiabilityCode: advanceAccountCode, LiabilityName: advanceAccountName,
		Reference: appID, Date: f.AsOf,
	}
	if err := v.ensureAdvanceAccount(ctx, f); err != nil {
		return s, err
	}
	if id, found, err := v.findAdvance(ctx, f, appID); err != nil {
		return s, err
	} else if found {
		s.BankTransactionID = id
		return s, nil
	}
	lender, err := v.ensureLender(ctx, f)
	if err != nil {
		return s, err
	}

	var out struct {
		BankTransactions []struct {
			BankTransactionID string `json:"BankTransactionID"`
		} `json:"BankTransactions"`
	}
	err = v.Xero.JSON(ctx, http.MethodPut, xeroBase+"/BankTransactions", xeroHeaders(f), map[string]any{
		"BankTransactions": []map[string]any{{
			"Type":        "RECEIVE",
			"Contact":     map[string]string{"ContactID": lender},
			"BankAccount": map[string]string{"Code": bankAccountCode},
			"Date":        f.AsOf,
			"Reference":   appID,
			"LineItems": []map[string]any{{
				"Description": advanceAccountName,
				"Quantity":    1,
				"UnitAmount":  float64(fund.AmountCents) / 100,
				"AccountCode": advanceAccountCode,
			}},
		}},
	}, &out)
	if err != nil {
		return s, fmt.Errorf("record the advance as money received in the merchant's books: %w", err)
	}
	if len(out.BankTransactions) == 0 {
		return s, fmt.Errorf("Xero did not return the bank transaction it created")
	}
	s.BankTransactionID = out.BankTransactions[0].BankTransactionID
	return s, nil
}
