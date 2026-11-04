package world

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

const (
	xeroPrefix = "/api.xro/2.0"
	contactsJS = `{"Contacts":[{"ContactID":"c0"},{"ContactID":"c1"},{"ContactID":"c2"}]}`
	invoicesJS = `{"Invoices":[{"InvoiceID":"i0"},{"InvoiceID":"i1"},{"InvoiceID":"i2"},{"InvoiceID":"i3"},{"InvoiceID":"i4"},{"InvoiceID":"i5"}]}`
)

func newFakeXero(t *testing.T) *fake {
	return newFake(t, map[string]string{
		"PUT " + xeroPrefix + "/Contacts":  contactsJS,
		"PUT " + xeroPrefix + "/Invoices":  invoicesJS,
		"POST " + xeroPrefix + "/Payments": `{}`,
	})
}

var asOf = time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)

func TestSeedXeroCreatesCustomersInvoicesAndPaymentsForThePaidOnes(t *testing.T) {
	f := newFakeXero(t)
	facts, err := SeedXero(context.Background(), nil, wire.Direct("xero", f.srv.URL), asOf)
	if err != nil {
		t.Fatal(err)
	}

	if len(f.to("POST", "/admin/time/advance")) != 1 {
		t.Fatal("the Xero clock should move to the world's date once")
	}
	var inv struct {
		Invoices []struct {
			Contact map[string]string
			Date    string
			DueDate string
		}
	}
	json.Unmarshal([]byte(f.to("PUT", xeroPrefix+"/Invoices")[0].Body), &inv)
	if len(inv.Invoices) != 6 || inv.Invoices[0].Contact["ContactID"] != "c0" || inv.Invoices[1].Contact["ContactID"] != "c1" {
		t.Fatalf("invoices should use the contacts Xero returned: %+v", inv.Invoices)
	}
	// 78 days before 2027-02-01 is 2026-11-15, due 30 days after.
	if inv.Invoices[0].Date != "2026-11-15" || inv.Invoices[0].DueDate != "2026-12-15" {
		t.Fatalf("dates should be relative to the world's date: %+v", inv.Invoices[0])
	}

	var pay struct {
		Payments []struct {
			Invoice map[string]string
			Account map[string]string
			Date    string
			Amount  int64
		}
	}
	json.Unmarshal([]byte(f.to("POST", xeroPrefix+"/Payments")[0].Body), &pay)
	if len(pay.Payments) != 3 {
		t.Fatalf("only the three paid invoices get a payment, got %d", len(pay.Payments))
	}
	for i, p := range pay.Payments {
		if p.Invoice["InvoiceID"] != []string{"i0", "i1", "i2"}[i] || p.Account["Code"] != "090" {
			t.Fatalf("payment %d is for the wrong invoice or account: %+v", i, p)
		}
		if p.Date >= asOf.Format("2006-01-02") {
			t.Fatalf("payment %d is dated %s, which is not before the world's date", i, p.Date)
		}
	}

	// The numbers the lender will see. Pinned so the story cannot drift quietly.
	if facts.Contacts != 3 || facts.Invoices != 6 || facts.BilledCents != 3450000 || facts.OutstandingCents != 1615000 || facts.OverdueCents != 540000 {
		t.Fatalf("unexpected facts: %+v", facts)
	}
}

func TestSeedXeroEveryUnpaidInvoiceIsAccountedForInOverdueOrNot(t *testing.T) {
	// One unpaid invoice is past due and two are inside their terms.
	var overdue, current int
	for _, in := range wholesaleInvoices {
		if in.paid {
			continue
		}
		if in.daysAgo > paymentTermsDays {
			overdue++
		} else {
			current++
		}
	}
	if overdue != 1 || current != 2 {
		t.Fatalf("want 1 overdue and 2 current unpaid invoices, got %d and %d", overdue, current)
	}
}

func TestSeedXeroSaysWhichStepFailed(t *testing.T) {
	f := newFake(t, map[string]string{"PUT " + xeroPrefix + "/Contacts": `{"Contacts":[]}`})
	_, err := SeedXero(context.Background(), nil, wire.Direct("xero", f.srv.URL), asOf)
	if err == nil || !strings.Contains(err.Error(), "expected 3") {
		t.Fatalf("a short answer should be caught and explained: %v", err)
	}
}
