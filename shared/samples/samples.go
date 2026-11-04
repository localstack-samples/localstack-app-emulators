// Package samples checks the descriptors of sample apps.
//
// A sample is an open source app, written by someone else for real users, that
// is run against the app emulators. This repo records which app, at which
// commit, and how it was pointed at the app emulators. It never holds the app's
// code: the code stays under its own license in its own repo, and a run fetches
// the pinned commit.
package samples

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// File is the name of a sample's descriptor.
const File = "sample.json"

// Descriptor says which app a sample is and how to run it against the app
// emulators.
type Descriptor struct {
	Name    string   `json:"name"`    // the directory name
	Repo    string   `json:"repo"`    // https URL of the app's repo
	Commit  string   `json:"commit"`  // the full commit SHA that was run
	License string   `json:"license"` // the app's SPDX license identifier
	Vendors []string `json:"vendors"` // the vendors whose app emulators it is run against
	// Overrides says, for each vendor, how the app is pointed at an app emulator:
	// the environment variable or setting that holds the vendor's base URL.
	Overrides map[string]string `json:"overrides"`
	Run       string            `json:"run"` // the command that runs the app or its tests, from the app's checkout
}

var (
	slug    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	sha     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	spdx    = regexp.MustCompile(`^[A-Za-z0-9.+-]+( (AND|OR|WITH) [A-Za-z0-9.+-]+)*$`)
	repoURL = regexp.MustCompile(`^https://[a-z0-9.-]+/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
)

// Validate lists what is wrong with d, given the directory name it sits in.
func (d Descriptor) Validate(dir string) []string {
	var p []string
	add := func(format string, args ...any) { p = append(p, fmt.Sprintf(format, args...)) }

	if d.Name != dir {
		add("name %q must be the directory name %q", d.Name, dir)
	}
	if !slug.MatchString(dir) {
		add("directory name %q must be lower-case words joined by hyphens", dir)
	}
	if !repoURL.MatchString(d.Repo) {
		add("repo %q must be an https URL of the form https://host/owner/name", d.Repo)
	}
	if !sha.MatchString(d.Commit) {
		add("commit %q must be a full 40-character lower-case SHA, not a branch or a tag", d.Commit)
	}
	if !validLicense(d.License) {
		add("license %q must be an SPDX identifier such as MIT or Apache-2.0", d.License)
	}
	if len(d.Vendors) == 0 {
		add("vendors must name at least one vendor")
	}
	seen := map[string]bool{}
	for _, v := range d.Vendors {
		if !slug.MatchString(v) {
			add("vendor %q must be lower-case words joined by hyphens", v)
		}
		if seen[v] {
			add("vendor %q is listed twice", v)
		}
		seen[v] = true
		if strings.TrimSpace(d.Overrides[v]) == "" {
			add("overrides must say how %s is pointed at its app emulator", v)
		}
	}
	for v := range d.Overrides {
		if !seen[v] {
			add("overrides names %q, which is not in vendors", v)
		}
	}
	if strings.TrimSpace(d.Run) == "" {
		add("run must be the command that runs the app or its tests")
	}
	sort.Strings(p)
	return p
}

// validLicense accepts an SPDX identifier or expression. The operators are not
// identifiers, so "OR" on its own is not a license.
func validLicense(l string) bool {
	if !spdx.MatchString(l) {
		return false
	}
	switch l {
	case "AND", "OR", "WITH":
		return false
	}
	return true
}

// allowed is what a sample's directory may hold. Anything else is app code or
// output that belongs elsewhere.
func allowed(name string, isDir bool) bool {
	switch {
	case name == File || name == "README.md":
		return !isDir
	case name == "results":
		return isDir
	}
	return false
}

// Problem is one thing wrong in the samples tree.
type Problem struct {
	Sample string // the sample's directory name, or "" for the tree itself
	Detail string
}

func (p Problem) String() string {
	if p.Sample == "" {
		return p.Detail
	}
	return p.Sample + ": " + p.Detail
}

// Check validates every sample under root. A sample is a directory holding
// anything at all, so one with no descriptor is a problem, not an omission.
func Check(root string) ([]Problem, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Problem
	for _, e := range entries {
		if !e.IsDir() {
			if e.Name() != "README.md" {
				out = append(out, Problem{Detail: fmt.Sprintf("%s is not a sample directory", e.Name())})
			}
			continue
		}
		out = append(out, checkSample(root, e.Name())...)
	}
	return out, nil
}

func checkSample(root, name string) []Problem {
	dir := filepath.Join(root, name)
	var out []Problem
	add := func(format string, args ...any) {
		out = append(out, Problem{Sample: name, Detail: fmt.Sprintf(format, args...)})
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		add("cannot read: %v", err)
		return out
	}
	for _, e := range entries {
		if !allowed(e.Name(), e.IsDir()) {
			add("%s does not belong here. A sample holds %s, README.md and results/, never the app's code", e.Name(), File)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		add("has no %s", File)
		return out
	}
	var d Descriptor
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		add("%s is not valid: %v", File, err)
		return out
	}
	for _, detail := range d.Validate(name) {
		add("%s", detail)
	}
	return out
}
