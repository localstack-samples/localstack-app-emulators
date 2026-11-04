package wire

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactJSONNested(t *testing.T) {
	in := `{"client_id":"cid","secret":"s","auth":{"access_token":"a","note":"keep"},"items":[{"refresh_token":"r","n":1}]}`
	out := string(redact("application/json; charset=utf-8", []byte(in)))
	for _, leaked := range []string{`"s"`, `"a"`, `"r"`} {
		if strings.Contains(out, leaked) {
			t.Fatalf("%s leaked: %s", leaked, out)
		}
	}
	for _, kept := range []string{`"cid"`, `"keep"`, `"n":1`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("%s should survive: %s", kept, out)
		}
	}
}

func TestRedactFormBody(t *testing.T) {
	in := "grant_type=refresh_token&refresh_token=abc&client_secret=xyz&scope=openid"
	out, _ := url.ParseQuery(string(redact("application/x-www-form-urlencoded", []byte(in))))
	if out.Get("refresh_token") != redacted || out.Get("client_secret") != redacted {
		t.Fatalf("secrets not masked: %v", out)
	}
	if out.Get("grant_type") != "refresh_token" || out.Get("scope") != "openid" {
		t.Fatalf("other fields must survive: %v", out)
	}
}

func TestRedactLeavesOtherBodiesAndBadJSONAlone(t *testing.T) {
	for _, tc := range []struct{ ct, body string }{
		{"text/plain", "secret=hello"},
		{"application/json", "{not json"},
		{"", ""},
	} {
		if got := string(redact(tc.ct, []byte(tc.body))); got != tc.body {
			t.Errorf("redact(%q, %q) = %q, want it unchanged", tc.ct, tc.body, got)
		}
	}
}

func TestSensitiveMatchesSuffixesNotSubstrings(t *testing.T) {
	for key, want := range map[string]bool{
		"secret": true, "Client_Secret": true, "plaid_secret": true, "access_token": true,
		"secretary": false, "token_count": false, "customer": false,
	} {
		if got := sensitive(key); got != want {
			t.Errorf("sensitive(%q) = %v, want %v", key, got, want)
		}
	}
}
