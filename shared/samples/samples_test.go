package samples

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func good() Descriptor {
	return Descriptor{
		Name:      "medusa-checkout",
		Repo:      "https://github.com/example/shop",
		Commit:    strings.Repeat("a", 40),
		License:   "MIT",
		Vendors:   []string{"stripe"},
		Overrides: map[string]string{"stripe": "STRIPE_API_BASE"},
		Run:       "npm test",
	}
}

func TestAGoodDescriptorHasNoProblems(t *testing.T) {
	if p := good().Validate("medusa-checkout"); len(p) != 0 {
		t.Fatalf("unexpected problems: %v", p)
	}
}

func TestValidateNamesEachProblem(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Descriptor)
		dir    string
		want   string
	}{
		{"name must match the directory", func(d *Descriptor) { d.Name = "other" }, "medusa-checkout", `must be the directory name`},
		{"directory is a slug", func(d *Descriptor) { d.Name = "Bad_Name" }, "Bad_Name", `lower-case words`},
		{"repo is https", func(d *Descriptor) { d.Repo = "git@github.com:example/shop.git" }, "medusa-checkout", `https URL`},
		{"a branch is not a commit", func(d *Descriptor) { d.Commit = "main" }, "medusa-checkout", `full 40-character`},
		{"a short SHA is not a commit", func(d *Descriptor) { d.Commit = "abc1234" }, "medusa-checkout", `full 40-character`},
		{"an upper-case SHA is rejected", func(d *Descriptor) { d.Commit = strings.Repeat("A", 40) }, "medusa-checkout", `full 40-character`},
		{"license is required", func(d *Descriptor) { d.License = "" }, "medusa-checkout", `SPDX`},
		{"vendors are required", func(d *Descriptor) { d.Vendors, d.Overrides = nil, nil }, "medusa-checkout", `at least one vendor`},
		{"every vendor needs an override", func(d *Descriptor) { d.Overrides = map[string]string{} }, "medusa-checkout", `how stripe is pointed`},
		{"a blank override does not count", func(d *Descriptor) { d.Overrides["stripe"] = "  " }, "medusa-checkout", `how stripe is pointed`},
		{"an override needs a vendor", func(d *Descriptor) { d.Overrides["plaid"] = "PLAID_HOST" }, "medusa-checkout", `"plaid", which is not in vendors`},
		{"a vendor twice", func(d *Descriptor) { d.Vendors = []string{"stripe", "stripe"} }, "medusa-checkout", `listed twice`},
		{"run is required", func(d *Descriptor) { d.Run = "" }, "medusa-checkout", `command that runs`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := good()
			tc.mutate(&d)
			got := strings.Join(d.Validate(tc.dir), "\n")
			if !strings.Contains(got, tc.want) {
				t.Fatalf("want a problem containing %q, got:\n%s", tc.want, got)
			}
		})
	}
}

func TestLicenseAcceptsSPDXExpressions(t *testing.T) {
	for _, l := range []string{"MIT", "Apache-2.0", "GPL-3.0-or-later", "MIT OR Apache-2.0"} {
		d := good()
		d.License = l
		if p := d.Validate("medusa-checkout"); len(p) != 0 {
			t.Errorf("%q should be accepted: %v", l, p)
		}
	}
	for _, l := range []string{"see LICENSE file", "MIT;rm", "OR"} {
		d := good()
		d.License = l
		if p := d.Validate("medusa-checkout"); len(p) == 0 {
			t.Errorf("%q should be rejected", l)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func descriptorJSON(t *testing.T, d Descriptor) string {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCheckAcceptsAWellFormedTree(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# samples")
	write(t, filepath.Join(root, "medusa-checkout", File), descriptorJSON(t, good()))
	write(t, filepath.Join(root, "medusa-checkout", "README.md"), "notes")
	write(t, filepath.Join(root, "medusa-checkout", "results", "2026-11-04.json"), "{}")
	if p, err := Check(root); err != nil || len(p) != 0 {
		t.Fatalf("err=%v problems=%v", err, p)
	}
}

func TestCheckAcceptsNoSamples(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "README.md"), "# samples")
	if p, err := Check(root); err != nil || len(p) != 0 {
		t.Fatalf("an empty section is fine: err=%v problems=%v", err, p)
	}
}

// The reason a sample holds no app code is the app's license, so a checkout
// that slips in must fail the check.
func TestCheckRejectsAppCodeInASample(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "medusa-checkout", File), descriptorJSON(t, good()))
	write(t, filepath.Join(root, "medusa-checkout", "src", "app.ts"), "export {}")
	write(t, filepath.Join(root, "medusa-checkout", "package.json"), "{}")
	p, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, x := range p {
		got = append(got, x.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"src does not belong here", "package.json does not belong here"} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %q in:\n%s", want, joined)
		}
	}
}

func TestCheckReportsMissingAndMalformedDescriptors(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "no-descriptor", "README.md"), "x")
	write(t, filepath.Join(root, "bad-json", File), "{not json")
	write(t, filepath.Join(root, "extra-field", File), `{"name":"extra-field","surprise":1}`)
	p, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, x := range p {
		got = append(got, x.String())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"no-descriptor: has no sample.json", "bad-json: sample.json is not valid", "extra-field: sample.json is not valid"} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %q in:\n%s", want, joined)
		}
	}
}

func TestCheckRejectsStrayFilesAtTheTop(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "notes.txt"), "x")
	p, _ := Check(root)
	if len(p) != 1 || !strings.Contains(p[0].String(), "notes.txt is not a sample directory") {
		t.Fatalf("got %v", p)
	}
}

func TestCheckFailsWhenTheRootIsMissing(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("a missing samples directory should be an error")
	}
}
