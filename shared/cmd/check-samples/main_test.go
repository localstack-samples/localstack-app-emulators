package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExitCodes(t *testing.T) {
	good := t.TempDir()
	bad := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bad, "no-descriptor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "no-descriptor", "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		code int
		out  string
		err  string
	}{
		{"an empty section passes", []string{good}, 0, "well formed", ""},
		{"a bad sample fails and is named", []string{bad}, 1, "", "no-descriptor: has no sample.json"},
		{"a missing directory is an error", []string{filepath.Join(good, "nope")}, 2, "", "check-samples:"},
		{"too many arguments", []string{"a", "b"}, 2, "", "usage:"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if got := run(tc.args, &out, &errb); got != tc.code {
				t.Fatalf("exit %d, want %d (stderr: %s)", got, tc.code, errb.String())
			}
			if !strings.Contains(out.String(), tc.out) || !strings.Contains(errb.String(), tc.err) {
				t.Fatalf("stdout %q, stderr %q", out.String(), errb.String())
			}
		})
	}
}
