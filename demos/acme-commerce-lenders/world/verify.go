package world

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
)

// Stale says one app emulator no longer holds the world the lender saved. It
// happens when an app emulator restarts or is reset by something else: it
// answers its health check, but the Plaid access token, the connected Stripe
// account or the Xero invoices the facts name are gone.
type Stale struct {
	Vendor string `json:"vendor"`
	Reason string `json:"reason"`
}

// Verify checks that each app emulator still holds what f says seeding put in
// it, and returns one Stale for each that does not.
//
// Only a definite answer counts. An app emulator that cannot be reached, or
// answers with a server error, is not reported here, because the readiness
// check already says so and this one must not call a slow app emulator stale.
func Verify(ctx context.Context, hc *http.Client, tg Targets, f Facts) []Stale {
	var out []Stale
	add := func(vendor string, err error) {
		var e *seed.Error
		if errors.As(err, &e) && e.Status >= 400 && e.Status < 500 {
			out = append(out, Stale{Vendor: vendor, Reason: fmt.Sprintf("the %s app emulator no longer holds the seeded world (%v)", vendor, err)})
		}
	}

	stripe := seed.API{HC: hc, Target: tg.Stripe}
	add("stripe", stripe.Do(ctx, http.MethodGet, "/v1/accounts/"+f.Stripe.AccountID,
		map[string]string{"Authorization": "Bearer " + StripeKey}, "", nil, nil))

	plaid := seed.API{HC: hc, Target: tg.Plaid}
	add("plaid", plaid.JSON(ctx, http.MethodPost, "/item/get", nil, map[string]any{
		"client_id": plaidClientID, "secret": plaidSecret, "access_token": f.Plaid.AccessToken,
	}, nil))

	xero := seed.API{HC: hc, Target: tg.Xero}
	var invoices struct {
		Invoices []struct{ InvoiceID string }
	}
	err := xero.Do(ctx, http.MethodGet, "/api.xro/2.0/Invoices",
		map[string]string{"Authorization": "Bearer demo", "Xero-Tenant-Id": f.Xero.TenantID}, "", nil, &invoices)
	add("xero", err)
	if err == nil && len(invoices.Invoices) != f.Xero.Invoices {
		out = append(out, Stale{Vendor: "xero", Reason: fmt.Sprintf(
			"the xero app emulator holds %d invoices, and seeding made %d", len(invoices.Invoices), f.Xero.Invoices)})
	}
	return out
}
