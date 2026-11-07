package world

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// XeroTenantID is the tenant the Xero app emulator answers to by default. The
// lender would learn it from Xero's connections endpoint after an OAuth sign-in.
const XeroTenantID = "00000000-0000-0000-0000-000000000001"

// XeroFacts describes what was seeded into the Xero app emulator.
type XeroFacts struct {
	TenantID         string `json:"tenant_id"`
	Contacts         int    `json:"contacts"`
	Invoices         int    `json:"invoices"`
	BilledCents      int64  `json:"billed_cents"`
	OutstandingCents int64  `json:"outstanding_cents"` // billed and not yet paid
	OverdueCents     int64  `json:"overdue_cents"`     // outstanding and past its due date as of the world's date
}

// The merchant also sells wholesale, which is what its invoices are for.
var wholesaleCustomers = []string{"Cascade Outfitters", "Ridgeline Supply Co.", "Summit Trail Stores"}

type invoiceSpec struct {
	customer int   // index into wholesaleCustomers
	daysAgo  int   // when it was issued, before the world's date
	dollars  int64 // the total
	paid     bool
}

const paymentTermsDays = 30

// One invoice is overdue and two are still inside their terms, so the lender
// sees receivables in each state.
var wholesaleInvoices = []invoiceSpec{
	{0, 78, 6200, true},
	{1, 64, 4850, true},
	{2, 49, 7300, true},
	{0, 33, 5400, false}, // due 3 days ago
	{1, 21, 3950, false},
	{2, 9, 6800, false},
}

// SeedXero gives the merchant a set of wholesale invoices, most of them paid.
// asOf is the date the world's history ends. The app emulator's clock is moved
// to it, so it agrees with Stripe's.
//
// The invoices use the Xero app emulator's revenue account 200. The app
// emulator cannot create accounts, so the chart is the one it starts with.
func SeedXero(ctx context.Context, hc *http.Client, t wire.Target, asOf time.Time) (XeroFacts, error) {
	facts := XeroFacts{TenantID: XeroTenantID}
	api := seed.API{HC: hc, Target: t}
	hdr := map[string]string{"Authorization": "Bearer demo", "Xero-Tenant-Id": XeroTenantID}
	const base = "/api.xro/2.0"

	if err := seed.AdvanceClock(ctx, hc, t, Days*24*time.Hour); err != nil {
		return facts, fmt.Errorf("advance the clock: %w", err)
	}

	contacts := make([]map[string]string, len(wholesaleCustomers))
	for i, name := range wholesaleCustomers {
		contacts[i] = map[string]string{"Name": name}
	}
	var contactsOut struct {
		Contacts []struct{ ContactID string }
	}
	if err := api.JSON(ctx, http.MethodPut, base+"/Contacts", hdr, map[string]any{"Contacts": contacts}, &contactsOut); err != nil {
		return facts, fmt.Errorf("create the wholesale customers: %w", err)
	}
	if len(contactsOut.Contacts) != len(wholesaleCustomers) {
		return facts, fmt.Errorf("created %d wholesale customers, expected %d", len(contactsOut.Contacts), len(wholesaleCustomers))
	}
	facts.Contacts = len(contactsOut.Contacts)

	day := func(daysAgo int) string { return asOf.AddDate(0, 0, -daysAgo).Format("2006-01-02") }
	invoices := make([]map[string]any, len(wholesaleInvoices))
	for i, in := range wholesaleInvoices {
		invoices[i] = map[string]any{
			"Type":    "ACCREC",
			"Contact": map[string]string{"ContactID": contactsOut.Contacts[in.customer].ContactID},
			"Date":    day(in.daysAgo),
			"DueDate": day(in.daysAgo - paymentTermsDays),
			"Status":  "AUTHORISED",
			"LineItems": []map[string]any{{
				"Description": "Wholesale order",
				"Quantity":    1,
				"UnitAmount":  in.dollars,
				"AccountCode": "200",
			}},
		}
	}
	var invoicesOut struct {
		Invoices []struct{ InvoiceID string }
	}
	if err := api.JSON(ctx, http.MethodPut, base+"/Invoices", hdr, map[string]any{"Invoices": invoices}, &invoicesOut); err != nil {
		return facts, fmt.Errorf("create the wholesale invoices: %w", err)
	}
	if len(invoicesOut.Invoices) != len(wholesaleInvoices) {
		return facts, fmt.Errorf("created %d invoices, expected %d", len(invoicesOut.Invoices), len(wholesaleInvoices))
	}

	var payments []map[string]any
	for i, in := range wholesaleInvoices {
		facts.Invoices++
		facts.BilledCents += in.dollars * 100
		if in.paid {
			payments = append(payments, map[string]any{
				"Invoice": map[string]string{"InvoiceID": invoicesOut.Invoices[i].InvoiceID},
				"Account": map[string]string{"Code": "090"},
				"Date":    day(in.daysAgo - 20),
				"Amount":  in.dollars,
			})
			continue
		}
		facts.OutstandingCents += in.dollars * 100
		if in.daysAgo > paymentTermsDays {
			facts.OverdueCents += in.dollars * 100
		}
	}
	if err := api.JSON(ctx, http.MethodPost, base+"/Payments", hdr, map[string]any{"Payments": payments}, nil); err != nil {
		return facts, fmt.Errorf("record the paid invoices' payments: %w", err)
	}
	return facts, nil
}
