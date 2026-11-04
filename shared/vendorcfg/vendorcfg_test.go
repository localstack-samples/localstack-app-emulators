package vendorcfg

import (
	"strings"
	"testing"
)

func TestEnvName(t *testing.T) {
	for vendor, want := range map[string]string{
		"stripe":   "STRIPE_BASE_URL",
		"Xero":     "XERO_BASE_URL",
		"logo-dev": "LOGO_DEV_BASE_URL",
	} {
		if got := EnvName(vendor); got != want {
			t.Errorf("EnvName(%q) = %q, want %q", vendor, got, want)
		}
	}
}

func TestFromEnvTrimsTrailingSlash(t *testing.T) {
	env := map[string]string{"STRIPE_BASE_URL": "http://127.0.0.1:5002/", "XERO_BASE_URL": "http://127.0.0.1:5001"}
	got := FromEnv(func(k string) string { return env[k] }, "stripe", "plaid", "xero")
	if got["stripe"] != "http://127.0.0.1:5002" || got["xero"] != "http://127.0.0.1:5001" {
		t.Fatalf("unexpected URLs: %+v", got)
	}
	if v, ok := got["plaid"]; !ok || v != "" {
		t.Fatalf("an unset vendor must be present and empty, got %q (present: %v)", v, ok)
	}
}

func TestFromEnvKnowsOnlyTheVendorsAsked(t *testing.T) {
	got := FromEnv(func(string) string { return "http://x" }, "github")
	if len(got) != 1 || got["github"] != "http://x" {
		t.Fatalf("unexpected URLs: %+v", got)
	}
}

func TestRequireRejectsEmptyAndNamesTheVariable(t *testing.T) {
	_, err := Require("xero", "")
	if err == nil || !strings.Contains(err.Error(), "XERO_BASE_URL") {
		t.Fatalf("want an error naming XERO_BASE_URL, got %v", err)
	}
	if v, err := Require("xero", "http://127.0.0.1:5001"); err != nil || v == "" {
		t.Fatalf("expected the URL back, got %q, %v", v, err)
	}
}

func TestEmulatorEnvName(t *testing.T) {
	for vendor, want := range map[string]string{"plaid": "PLAID_EMULATOR_URL", "logo-dev": "LOGO_DEV_EMULATOR_URL"} {
		if got := EmulatorEnvName(vendor); got != want {
			t.Errorf("EmulatorEnvName(%q) = %q, want %q", vendor, got, want)
		}
	}
}

func TestEmulatorVersionEnvName(t *testing.T) {
	if got := EmulatorVersionEnvName("plaid"); got != "PLAID_EMULATOR_VERSION" {
		t.Fatalf("got %q", got)
	}
	if got := EmulatorVersionEnvName("logo-dev"); got != "LOGO_DEV_EMULATOR_VERSION" {
		t.Fatalf("got %q", got)
	}
}
