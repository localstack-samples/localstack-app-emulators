// Command kit is the demo: one binary that serves the UI and, from the command
// line, checks, seeds and resets the world. It runs beside LocalStack, which
// serves the app emulators it includes. Any other app emulator you run yourself
// and name with <VENDOR>_EMULATOR_URL.
//
// Scaffold for the UI: it serves the placeholder page. The lender logic is not
// built yet.
package main

import (
	"context"
	"os"

	"github.com/localstack-samples/localstack-app-emulators/apps/acme-commerce-lenders/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
