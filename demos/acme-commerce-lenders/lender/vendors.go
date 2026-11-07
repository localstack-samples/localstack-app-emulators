// Package lender is ACME Commerce Lenders' own logic: it takes a merchant's
// application, connects to the merchant's Stripe, Plaid and Xero accounts,
// reads what they hold, and makes a working-capital offer.
//
// Every call the lender makes to a vendor goes through a wire proxy, so the
// inspector can show it.
package lender

import (
	"github.com/localstack-samples/localstack-app-emulators/demos/acme-commerce-lenders/world"
	"github.com/localstack-samples/localstack-app-emulators/shared/seed"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// Vendors are the lender's three vendor clients.
type Vendors struct {
	Stripe, Plaid, Xero seed.API
}

// NewVendors builds a client for each target. Each one sends its calls through
// a wire proxy that records them in rec, and the proxy forwards them to the
// app emulator, wherever it is served from.
func NewVendors(rec *wire.Recorder, tg world.Targets) Vendors {
	return Vendors{Stripe: apiFor(rec, tg.Stripe), Plaid: apiFor(rec, tg.Plaid), Xero: apiFor(rec, tg.Xero)}
}

// NewSwitchedVendors is NewVendors for targets that can change while the demo
// runs. Each call asks sw where its vendor is now.
func NewSwitchedVendors(rec *wire.Recorder, sw *wire.Switch) Vendors {
	lane := func(vendor string) seed.API {
		return seed.API{
			HC:     wire.NewSwitchedProxy(func() wire.Target { return sw.Target(vendor) }, rec).Client(),
			Target: wire.Target{Vendor: vendor, Upstream: "http://wire.invalid"},
		}
	}
	return Vendors{Stripe: lane("stripe"), Plaid: lane("plaid"), Xero: lane("xero")}
}

// apiFor returns a client whose calls the proxy records and forwards to t. The
// URL host is a placeholder, because the proxy uses its own target.
func apiFor(rec *wire.Recorder, t wire.Target) seed.API {
	return seed.API{
		HC:     wire.NewProxy(t, rec).Client(),
		Target: wire.Target{Vendor: t.Vendor, Source: t.Source, Upstream: "http://wire.invalid"},
	}
}

// Connection says whether the lender could reach one of the merchant's accounts.
type Connection struct {
	Vendor string `json:"vendor"`
	Detail string `json:"detail"`
}
