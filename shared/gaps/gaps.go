// Package gaps says which of the calls an app made are not covered by the app
// emulators it was run against.
//
// Coverage comes from each app emulator's twin-manifest.json, in
// WonderTwin-AI/wondertwin, under coverage.resources_implemented. It does not
// come from a response's status. An open source app emulator answers a call it
// lacks the way the real vendor answers an unknown URL (Stripe's returns a 404
// "Unrecognized request URL"), by design, so a 404 does not say which of the
// two it is.
//
// The manifests do not share a format, so each entry is read as one of three
// kinds and a call is matched the way its kind allows:
//
//	"PUT /api.xro/2.0/Accounts"  a route: method and path ({} matches one segment)
//	"item/get"                   a path: any method
//	"payment_intents"            a resource name: a path's first resource, any method
//
// A route answers for the call exactly. A path cannot tell methods apart. A
// resource name cannot tell operations apart, so "covered" means only that the
// app emulator implements some of that resource.
package gaps

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// Coverage is whether an app emulator covers a call.
type Coverage string

const (
	Covered   Coverage = "covered"
	Gap       Coverage = "gap"       // the app emulator's manifest does not list it
	Unchecked Coverage = "unchecked" // no manifest was given for the vendor
)

// Basis is how precisely a manifest can answer for a call.
type Basis string

const (
	BasisRoute    Basis = "route"    // method and path
	BasisPath     Basis = "path"     // path, any method
	BasisResource Basis = "resource" // resource name only
	BasisNone     Basis = ""
)

// Manifest is the coverage of one vendor's app emulator.
type Manifest struct {
	routes    []route
	paths     [][]string
	resources map[string]bool
}

type route struct {
	method string
	segs   []string
}

// LoadManifest reads coverage.resources_implemented from a twin-manifest.json.
func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var doc struct {
		Coverage struct {
			Implemented []string `json:"resources_implemented"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(doc.Coverage.Implemented) == 0 {
		return Manifest{}, fmt.Errorf("%s has no coverage.resources_implemented", path)
	}
	return ParseManifest(doc.Coverage.Implemented), nil
}

var methodPrefix = regexp.MustCompile(`^([A-Z]+) (/.*)$`)

// ParseManifest reads a list of resources_implemented entries.
func ParseManifest(entries []string) Manifest {
	m := Manifest{resources: map[string]bool{}}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		switch {
		case methodPrefix.MatchString(e):
			g := methodPrefix.FindStringSubmatch(e)
			m.routes = append(m.routes, route{method: g[1], segs: segments(g[2])})
		case strings.Contains(e, "/"):
			m.paths = append(m.paths, segments(e))
		case e != "":
			m.resources[e] = true
		}
	}
	return m
}

func segments(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func equalSegs(pattern, got []string) bool {
	if len(pattern) != len(got) {
		return false
	}
	for i := range pattern {
		if pattern[i] != "{}" && pattern[i] != got[i] {
			return false
		}
	}
	return true
}

// Covers reports whether the manifest covers a call, and how precisely it can
// say so. The path must not include a query string.
func (m Manifest) Covers(method, path string) (bool, Basis) {
	segs := segments(path)
	for _, r := range m.routes {
		if r.method == method && equalSegs(r.segs, segs) {
			return true, BasisRoute
		}
	}
	for _, p := range m.paths {
		if equalSegs(p, segs) {
			return true, BasisPath
		}
	}
	for _, name := range resourceNames(segs) {
		if m.resources[name] {
			return true, BasisResource
		}
	}
	// What the manifest could have answered for, so a gap says how sure it is.
	switch {
	case len(m.routes) > 0:
		return false, BasisRoute
	case len(m.paths) > 0:
		return false, BasisPath
	case len(m.resources) > 0:
		return false, BasisResource
	}
	return false, BasisNone
}

var (
	versionSeg = regexp.MustCompile(`^v[0-9]+$`)
	guid       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	digits     = regexp.MustCompile(`^[0-9]+$`)
	prefixedID = regexp.MustCompile(`^[a-z]{2,}_[A-Za-z0-9]{6,}$`)
	longToken  = regexp.MustCompile(`^[A-Za-z0-9]{20,}$`)
)

// idLike reports whether a path segment looks like an identifier rather than a
// resource name. It is a heuristic. Resource names are lower-case words, and
// the identifiers the vendors issue carry a digit or a capital, so a
// prefixed identifier must too.
func idLike(seg string) bool {
	switch {
	case guid.MatchString(seg), digits.MatchString(seg):
		return true
	case prefixedID.MatchString(seg):
		return strings.ToLower(seg) != seg || strings.ContainsAny(seg, "0123456789")
	case longToken.MatchString(seg):
		return true
	}
	return false
}

// Normalize replaces each identifier in a path with {}, so calls that differ
// only by identifier are one call.
func Normalize(path string) string {
	segs := segments(path)
	for i, s := range segs {
		if idLike(s) {
			segs[i] = "{}"
		}
	}
	return "/" + strings.Join(segs, "/")
}

// resourceNames lists the names a resource-level manifest could use for a
// path: with the version and identifiers dropped, the first resource, the last
// one and all of them joined by underscores. "/v1/accounts/acct_1/external_accounts"
// gives "accounts", "external_accounts" and "accounts_external_accounts".
func resourceNames(segs []string) []string {
	var words []string
	for i, s := range segs {
		if i == 0 && versionSeg.MatchString(s) {
			continue
		}
		if idLike(s) {
			continue
		}
		words = append(words, s)
	}
	if len(words) == 0 {
		return nil
	}
	names := []string{words[0], words[len(words)-1], strings.Join(words, "_")}
	if len(words) >= 2 {
		names = append(names, words[0]+"_"+words[1])
	}
	return names
}

// Call is one distinct call the app made, with how often.
type Call struct {
	Vendor   string   `json:"vendor"`
	Method   string   `json:"method"`
	Path     string   `json:"path"` // identifiers replaced with {}
	Count    int      `json:"count"`
	Failed   int      `json:"failed"` // answered with 4xx or 5xx, or not answered
	Coverage Coverage `json:"coverage"`
	Basis    Basis    `json:"basis,omitempty"`
}

// Report is what a run of an app asked of the app emulators.
type Report struct {
	Calls []Call `json:"calls"`
	// Versions lists the app emulator releases the calls were made against, per
	// vendor, so a report can be reproduced. "unknown" means none was recorded.
	Versions map[string][]string `json:"versions"`
}

// Gaps is the number of distinct calls the app emulators do not cover.
func (r Report) Gaps() int {
	n := 0
	for _, c := range r.Calls {
		if c.Coverage == Gap {
			n++
		}
	}
	return n
}

// Build checks each recorded call against its vendor's manifest. A vendor with
// no manifest is Unchecked, never a gap: the report does not guess.
func Build(events []wire.Event, manifests map[string]Manifest) Report {
	type key struct{ vendor, method, path string }
	calls := map[key]*Call{}
	versions := map[string]map[string]bool{}

	for _, e := range events {
		path := e.Path
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		k := key{e.Vendor, e.Method, Normalize(path)}
		c, ok := calls[k]
		if !ok {
			c = &Call{Vendor: e.Vendor, Method: e.Method, Path: k.path, Coverage: Unchecked}
			if m, have := manifests[e.Vendor]; have {
				covered, basis := m.Covers(e.Method, path)
				c.Basis = basis
				c.Coverage = Gap
				if covered {
					c.Coverage = Covered
				}
			}
			calls[k] = c
		}
		c.Count++
		if e.Error != "" || e.Status >= 400 {
			c.Failed++
		}

		if versions[e.Vendor] == nil {
			versions[e.Vendor] = map[string]bool{}
		}
		v := e.EmulatorVersion
		if v == "" {
			v = "unknown"
		}
		versions[e.Vendor][v] = true
	}

	r := Report{Versions: map[string][]string{}}
	for _, c := range calls {
		r.Calls = append(r.Calls, *c)
	}
	// Gaps first, since they are what the report is for.
	order := map[Coverage]int{Gap: 0, Unchecked: 1, Covered: 2}
	sort.Slice(r.Calls, func(i, j int) bool {
		a, b := r.Calls[i], r.Calls[j]
		if order[a.Coverage] != order[b.Coverage] {
			return order[a.Coverage] < order[b.Coverage]
		}
		if a.Vendor != b.Vendor {
			return a.Vendor < b.Vendor
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Method < b.Method
	})
	for vendor, set := range versions {
		for v := range set {
			r.Versions[vendor] = append(r.Versions[vendor], v)
		}
		sort.Strings(r.Versions[vendor])
	}
	return r
}
