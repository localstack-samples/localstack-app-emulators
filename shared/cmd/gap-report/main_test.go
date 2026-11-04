package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtures(t *testing.T) (events, xero, stripe string) {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	events = write("calls.json", `{"last":3,"generation":0,"events":[
	  {"id":1,"vendor":"xero","method":"PUT","path":"/api.xro/2.0/Accounts","status":200,"emulator_version":"aaaaaaaaaaaa"},
	  {"id":2,"vendor":"xero","method":"POST","path":"/api.xro/2.0/Accounts","status":405,"emulator_version":"aaaaaaaaaaaa"},
	  {"id":3,"vendor":"plaid","method":"POST","path":"/item/get","status":200}]}`)
	xero = write("xero.json", `{"coverage":{"resources_implemented":["PUT /api.xro/2.0/Accounts"]}}`)
	stripe = write("stripe.json", `{"coverage":{"resources_implemented":["customers"]}}`)
	return events, xero, stripe
}

func TestReportPrintsGapsAndExitsZero(t *testing.T) {
	events, xero, _ := fixtures(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--events", events, "--manifest", "xero=" + xero}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"xero:POST /api.xro/2.0/Accounts", "unchecked", "plaid:POST /item/get", "1 not covered"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("want %q in:\n%s", want, out.String())
		}
	}
}

func TestFailOnGap(t *testing.T) {
	events, xero, _ := fixtures(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--events", events, "--manifest", "xero=" + xero, "--fail-on-gap"}, &out, &errb); code != 1 {
		t.Fatalf("a gap should exit 1, got %d", code)
	}
	// With nothing to check against there is no gap to fail on.
	if code := run([]string{"--events", events, "--fail-on-gap"}, &out, &errb); code != 0 {
		t.Fatalf("unchecked calls are not gaps, got exit %d", code)
	}
}

func TestJSONOutput(t *testing.T) {
	events, xero, _ := fixtures(t)
	var out, errb bytes.Buffer
	if code := run([]string{"--events", events, "--manifest", "xero=" + xero, "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var r struct {
		Calls []struct {
			Vendor, Method, Path, Coverage string
		}
		Versions map[string][]string
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if len(r.Calls) != 3 || r.Calls[0].Coverage != "gap" || r.Versions["xero"][0] != "aaaaaaaaaaaa" {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestBadInputExitsTwo(t *testing.T) {
	events, xero, _ := fixtures(t)
	dir := t.TempDir()
	notRecording := filepath.Join(dir, "x.json")
	os.WriteFile(notRecording, []byte("{nope"), 0o644)
	empty := filepath.Join(dir, "empty.json")
	os.WriteFile(empty, []byte(`{"coverage":{"resources_implemented":[]}}`), 0o644)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no events", nil, "usage:"},
		{"a stray argument", []string{"--events", events, "extra"}, "usage:"},
		{"manifest without a path", []string{"--events", events, "--manifest", "xero"}, "vendor=path"},
		{"the same vendor twice", []string{"--events", events, "--manifest", "xero=" + xero, "--manifest", "xero=" + xero}, "twice"},
		{"a missing events file", []string{"--events", filepath.Join(dir, "absent.json")}, "gap-report:"},
		{"events that are not a recording", []string{"--events", notRecording}, "not a recording"},
		{"a manifest with no coverage", []string{"--events", events, "--manifest", "xero=" + empty}, "xero manifest"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
			if !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("stderr %q should contain %q", errb.String(), tc.want)
			}
		})
	}
}
