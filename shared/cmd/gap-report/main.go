// Command gap-report says which calls an app made that the app emulators do
// not cover.
//
//	gap-report --events calls.json --manifest stripe=twin-stripe/twin-manifest.json
//
// calls.json is what wire-proxy --out wrote. Each --manifest is an app
// emulator's twin-manifest.json from WonderTwin-AI/wondertwin. A vendor with no
// manifest is reported as unchecked, not as a gap.
//
// It exits 0 after printing the report. With --fail-on-gap it exits 1 when any
// call is not covered, and 2 when its input cannot be read.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/localstack-samples/localstack-app-emulators/shared/gaps"
	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

const usage = `usage: gap-report --events calls.json --manifest vendor=path [--manifest vendor=path ...] [--json] [--fail-on-gap]
`

type manifestFlags map[string]string

func (m manifestFlags) String() string { return "" }

func (m manifestFlags) Set(v string) error {
	vendor, path, ok := strings.Cut(v, "=")
	if !ok || vendor == "" || path == "" {
		return fmt.Errorf("--manifest %q must be vendor=path", v)
	}
	if _, dup := m[vendor]; dup {
		return fmt.Errorf("--manifest names %s twice", vendor)
	}
	m[vendor] = path
	return nil
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gap-report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	paths := manifestFlags{}
	events := fs.String("events", "", "")
	asJSON := fs.Bool("json", false, "")
	failOnGap := fs.Bool("fail-on-gap", false, "")
	fs.Var(paths, "manifest", "")
	if err := fs.Parse(args); err != nil || *events == "" || fs.NArg() > 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	raw, err := os.ReadFile(*events)
	if err != nil {
		fmt.Fprintf(stderr, "gap-report: %v\n", err)
		return 2
	}
	var rec struct {
		Events []wire.Event `json:"events"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		fmt.Fprintf(stderr, "gap-report: %s is not a recording: %v\n", *events, err)
		return 2
	}

	manifests := map[string]gaps.Manifest{}
	for vendor, path := range paths {
		m, err := gaps.LoadManifest(path)
		if err != nil {
			fmt.Fprintf(stderr, "gap-report: %s manifest: %v\n", vendor, err)
			return 2
		}
		manifests[vendor] = m
	}

	report := gaps.Build(rec.Events, manifests)
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		gaps.Write(stdout, report)
	}
	if *failOnGap && report.Gaps() > 0 {
		return 1
	}
	return 0
}
