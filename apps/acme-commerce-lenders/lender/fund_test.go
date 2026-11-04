package lender

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// offered drives an application up to an approved offer.
func offered(t *testing.T, svc *Service) Application {
	t.Helper()
	ctx := context.Background()
	a, err := svc.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []func(context.Context, string) (Application, error){svc.Connect, svc.Review, svc.Offer} {
		if a, err = step(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func TestAcceptedAdvanceIsPaidToThePlaidVerifiedAccount(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a := offered(t, svc)

	var err error
	if a, err = svc.Accept(ctx, a.ID); err != nil || a.Status != Accepted {
		t.Fatalf("accept: %+v, %v", a, err)
	}
	if a, err = svc.Fund(ctx, a.ID); err != nil || a.Status != Funded {
		t.Fatalf("fund: %+v, %v", a, err)
	}

	want := Funding{AmountCents: 3000000, PlaidAccountID: "chk", BankAccountID: "ba_plaid", BankLast4: "0000", RoutingNumber: "021000021", TransferID: "tr_advance", PayoutID: "po_advance", PayoutStatus: "pending"}
	if *a.Funding != want {
		t.Fatalf("funding:\n got %+v\nwant %+v", *a.Funding, want)
	}

	// The bank account Stripe was given is the one Plaid verified, in this order.
	if got := strings.Join(f.order, ","); got != "external,transfer,payout" {
		t.Fatalf("steps happened in the wrong order: %s", got)
	}
	ext := f.forms["external"]
	if ext.Get("external_account[routing_number]") != "021000021" || ext.Get("external_account[account_number]") != "1111222233330000" {
		t.Fatalf("Stripe should be given the numbers Plaid holds for the verified account: %v", ext)
	}
	if tr := f.forms["transfer"]; tr.Get("amount") != "3000000" || tr.Get("destination") != "acct_m" || tr.Get("metadata[application]") != a.ID {
		t.Fatalf("transfer: %v", tr)
	}
	if f.headers["payout"].Get("Stripe-Account") != "acct_m" || f.forms["payout"].Get("amount") != "3000000" {
		t.Fatalf("the payout is made on the merchant's own account: %v %v", f.headers["payout"], f.forms["payout"])
	}
	if f.headers["transfer"].Get("Idempotency-Key") != a.ID+"-transfer" || f.headers["payout"].Get("Idempotency-Key") != a.ID+"-payout" {
		t.Fatalf("money-moving calls must carry idempotency keys: %v %v", f.headers["transfer"], f.headers["payout"])
	}
}

// The account number is read to register the bank account and then forgotten.
func TestTheAccountNumberIsNeverKept(t *testing.T) {
	ctx := context.Background()
	svc, _, _, store := newService(t, true)
	a := offered(t, svc)
	svc.Accept(ctx, a.ID)
	svc.Fund(ctx, a.ID)

	raw, err := store.Get(ctx, "applications/"+a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "1111222233330000") {
		t.Fatalf("the full account number was stored: %s", raw)
	}
	var back Application
	json.Unmarshal(raw, &back)
	if back.Funding.BankLast4 != "0000" {
		t.Fatalf("only the last four digits are kept: %+v", back.Funding)
	}
}

func TestAFundingThatFailedHalfwayResumesWithoutMovingMoneyTwice(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a := offered(t, svc)
	svc.Accept(ctx, a.ID)
	f.failPayouts = 1 // the payout fails once, after the transfer went through

	got, err := svc.Fund(ctx, a.ID)
	if err == nil || !strings.Contains(err.Error(), "pay the advance out") {
		t.Fatalf("the failed payout should be reported: %v", err)
	}
	if got.Status != Accepted || got.Funding.TransferID != "tr_advance" || got.Funding.PayoutID != "" {
		t.Fatalf("a halfway funding keeps what finished: %+v", got)
	}
	if saved, _ := svc.Get(ctx, a.ID); saved.Funding == nil || saved.Funding.TransferID != "tr_advance" {
		t.Fatalf("the progress must be saved: %+v", saved)
	}

	got, err = svc.Fund(ctx, a.ID)
	if err != nil || got.Status != Funded || got.Funding.PayoutID != "po_advance" {
		t.Fatalf("the retry should finish the funding: %+v, %v", got, err)
	}
	if f.count("external") != 1 || f.count("transfer") != 1 || f.count("payout") != 2 {
		t.Fatalf("the bank account and transfer must not be repeated: external=%d transfer=%d payout=%d", f.count("external"), f.count("transfer"), f.count("payout"))
	}
}

func TestADeclinedOfferCannotBeAccepted(t *testing.T) {
	ctx := context.Background()
	svc, f, _, _ := newService(t, true)
	a, _ := svc.Apply(ctx)
	svc.Connect(ctx, a.ID) // the account is there when the lender connects
	f.noChecking = true    // and has gone by the time the lender reviews it, so it cannot be verified
	svc.Review(ctx, a.ID)
	if a, _ = svc.Offer(ctx, a.ID); a.Offer.Approved {
		t.Fatal("the offer should be declined when the bank account cannot be verified")
	}

	if _, err := svc.Accept(ctx, a.ID); !errors.Is(err, ErrDeclined) {
		t.Fatalf("want ErrDeclined, got %v", err)
	}
	if got, _ := svc.Get(ctx, a.ID); got.Status != Offered {
		t.Fatalf("a refused accept leaves the application offered: %s", got.Status)
	}
}

func TestFundingStepsAreOrdered(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newService(t, true)
	a, _ := svc.Apply(ctx)
	var wrong *WrongStateError
	if _, err := svc.Accept(ctx, a.ID); !errors.As(err, &wrong) {
		t.Fatalf("accept before an offer: %v", err)
	}
	if _, err := svc.Fund(ctx, a.ID); !errors.As(err, &wrong) || wrong.Want != Accepted {
		t.Fatalf("fund before accept: %v", err)
	}
}

// The inspector shows recorded calls on a projector, so the account number must
// not be in any of them, in either direction.
func TestNoRecordedCallShowsTheAccountNumber(t *testing.T) {
	ctx := context.Background()
	svc, _, rec, _ := newService(t, true)
	a := offered(t, svc)
	svc.Accept(ctx, a.ID)
	if _, err := svc.Fund(ctx, a.ID); err != nil {
		t.Fatal(err)
	}

	sawAuth, sawBankAccount := false, false
	for _, e := range rec.Events() {
		if e.Path == "/auth/get" {
			sawAuth = true
		}
		if strings.HasSuffix(e.Path, "/external_accounts") {
			sawBankAccount = true
		}
		for name, b := range map[string]string{"request": e.Request.Content, "response": e.Response.Content} {
			if strings.Contains(b, "1111222233330000") {
				t.Errorf("the account number is in the %s of %s %s: %s", name, e.Method, e.Path, b)
			}
		}
	}
	if !sawAuth || !sawBankAccount {
		t.Fatalf("the test must cover both calls that carry the number: auth=%v bank account=%v", sawAuth, sawBankAccount)
	}
}
