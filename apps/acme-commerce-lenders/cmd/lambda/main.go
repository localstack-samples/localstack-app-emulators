// Command lambda is the LocalStack mode entry point: the lender logic behind
// API Gateway and Lambda, with the app emulators reached over the network.
//
// Scaffold only. It does nothing yet; the Lambda handlers wrap package core.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "acme-lambda: LocalStack mode is not implemented yet")
	os.Exit(1)
}
