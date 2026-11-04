// Command kit is the offline kit: one binary that serves the UI, runs the
// lender logic and, once implemented, starts the Stripe, Plaid and Xero app
// emulators itself. It needs no network and no LocalStack.
//
// Scaffold only. It serves the placeholder UI; starting the app emulators,
// seeding the shared world and the lender logic are not implemented yet.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/internal/ui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address the UI listens on")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           ui.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("ACME Commerce Lenders offline kit (scaffold) on http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}
