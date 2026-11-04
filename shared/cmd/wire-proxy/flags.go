package main

import (
	"flag"
	"io"
)

// newFlagSet returns a flag set that reports errors to w and leaves the
// usage text to the caller.
func newFlagSet(w io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("wire-proxy", flag.ContinueOnError)
	fs.SetOutput(w)
	fs.Usage = func() {}
	return fs
}
