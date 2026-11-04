// Command check-samples validates the descriptor of every sample app.
//
//	check-samples [samples-dir]
//
// It exits 0 when every sample is well formed, 1 when any is not, and 2 when
// the directory cannot be read.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/localstack-samples/localstack-app-emulators/shared/samples"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: check-samples [samples-dir]")
		return 2
	}
	root := "samples"
	if len(args) == 1 {
		root = args[0]
	}
	problems, err := samples.Check(root)
	if err != nil {
		fmt.Fprintf(stderr, "check-samples: %v\n", err)
		return 2
	}
	for _, p := range problems {
		fmt.Fprintln(stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "%d problem(s) in %s\n", len(problems), root)
		return 1
	}
	fmt.Fprintf(stdout, "samples in %s are well formed\n", root)
	return 0
}
