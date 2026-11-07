package lender

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// funded drives an application to the point where the advance has been paid out.
func funded(t *testing.T, svc *Service) Application {
	t.Helper()
	ctx := context.Background()
	a := offered(t, svc)
	if _, err := svc.Accept(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	a, err := svc.Fund(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestSettlingBooksTheAdvanceAsMoneyReceivedAgainstALoanAccount(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a := funded(t, svc)

	a, err := svc.Settle(ctx, a.ID)
	if err != nil || a.Status != Settled {
		t.Fatalf("settle: %+v, %v", a, err)
	}
	want := Settlement{AmountCents: 3000000, BankTransactionID: "bt_advance", BankAccountCode: "090", LiabilityCode: "810", LiabilityName: "Working capital advance", Reference: a.ID, Date: "2027-02-01"}
	if *a.Settlement != want {
		t.Fatalf("settlement:\n got %+v\nwant %+v", *a.Settlement, want)
	}

	// The chart gets a loan account, and the bank transaction uses it.
	var acct struct{ Code, Name, Type string }
	json.Unmarshal(f.bodies["account"], &acct)
	if acct.Code != "810" || acct.Type != "CURRLIAB" {
		t.Fatalf("the loan account should be a current liability: %+v", acct)
	}
	var tx struct {
		BankTransactions []struct {
			Type        string
			Date        string
			Reference   string
			Contact     map[string]string
			BankAccount map[string]string
			LineItems   []struct {
				UnitAmount  float64
				AccountCode string
			}
		}
	}
	json.Unmarshal(f.bodies["banktx"], &tx)
	b := tx.BankTransactions[0]
	if b.Type != "RECEIVE" || b.BankAccount["Code"] != "090" || b.Contact["ContactID"] != "c-lender" || b.Reference != a.ID || b.Date != "2027-02-01" {
		t.Fatalf("the advance is money received into the bank account, from the lender: %+v", b)
	}
	if len(b.LineItems) != 1 || b.LineItems[0].UnitAmount != 30000 || b.LineItems[0].AccountCode != "810" {
		t.Fatalf("the line is $30,000 coded to the loan account: %+v", b.LineItems)
	}
}

func TestSettlingUsesAnExistingLoanAccountAndLenderContact(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	f.xeroAccounts = []string{"090", "810"}
	f.xeroContacts = []map[string]string{{"ContactID": "c-existing", "Name": "ACME Commerce Lenders"}}
	a := funded(t, svc)

	if _, err := svc.Settle(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if f.count("account") != 0 || f.count("contact") != 0 || f.count("banktx") != 1 {
		t.Fatalf("an account and a contact that exist must not be created again: account=%d contact=%d banktx=%d", f.count("account"), f.count("contact"), f.count("banktx"))
	}
	var tx struct {
		BankTransactions []struct{ Contact map[string]string }
	}
	json.Unmarshal(f.bodies["banktx"], &tx)
	if tx.BankTransactions[0].Contact["ContactID"] != "c-existing" {
		t.Fatalf("the existing contact should be used: %+v", tx.BankTransactions[0].Contact)
	}
}

func TestAFailedSettlementSavesNothingAndRetriesWithoutRecordingTwice(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a := funded(t, svc)
	f.failBankTx = 1

	if _, err := svc.Settle(ctx, a.ID); err == nil || !strings.Contains(err.Error(), "record the advance") {
		t.Fatalf("the failure should say what failed: %v", err)
	}
	if got, _ := svc.Get(ctx, a.ID); got.Status != Funded || got.Settlement != nil {
		t.Fatalf("a failed step leaves the application funded: %+v", got)
	}

	got, err := svc.Settle(ctx, a.ID)
	if err != nil || got.Status != Settled {
		t.Fatalf("the retry should work: %+v, %v", got, err)
	}
	if f.count("account") != 1 || f.count("contact") != 1 {
		t.Fatalf("the account and contact are created once across both tries: account=%d contact=%d", f.count("account"), f.count("contact"))
	}
	if n := len(f.bankTxs); n != 1 {
		t.Fatalf("one bank transaction in the books, got %d", n)
	}
}

// If the books already hold this application's transaction, such as after a lost
// answer, settling finds it instead of recording the advance twice.
func TestSettlingFindsItsOwnEarlierRecord(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a := funded(t, svc)
	f.xeroAccounts = []string{"810"}
	f.bankTxs = []map[string]string{{"BankTransactionID": "bt_earlier", "Reference": a.ID}}

	got, err := svc.Settle(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Settlement.BankTransactionID != "bt_earlier" || f.count("banktx") != 0 {
		t.Fatalf("the earlier record should be reused: %+v, banktx calls=%d", got.Settlement, f.count("banktx"))
	}
}

func TestSettlingNeedsAFundedApplication(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newService(t, true)
	a := offered(t, svc)
	var wrong *WrongStateError
	if _, err := svc.Settle(ctx, a.ID); !errors.As(err, &wrong) || wrong.Want != Funded {
		t.Fatalf("settle before funding: %v", err)
	}
}
