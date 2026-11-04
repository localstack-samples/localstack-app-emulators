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

func TestRedactMasksBankAccountNumbersButNotRoutingNumbers(t *testing.T) {
	form := "external_account%5Baccount_number%5D=1111222233330000&external_account%5Brouting_number%5D=021000021&external_account%5Bobject%5D=bank_account"
	out := string(redact("application/x-www-form-urlencoded", []byte(form)))
	if strings.Contains(out, "1111222233330000") {
		t.Fatalf("the account number leaked: %s", out)
	}
	if !strings.Contains(out, "021000021") || !strings.Contains(out, "bank_account") {
		t.Fatalf("the routing number and the rest should survive: %s", out)
	}

	plaid := `{"numbers":{"ach":[{"account_id":"acc1","account":"1111222233330000","routing":"021000021"}]}}`
	out = string(redact("application/json", []byte(plaid)))
	if strings.Contains(out, "1111222233330000") || !strings.Contains(out, "021000021") || !strings.Contains(out, "acc1") {
		t.Fatalf("Plaid's account number is masked and its routing number and id are not: %s", out)
	}
}

func TestRedactLeavesABareAccountAloneWhenThereIsNoRouting(t *testing.T) {
	out := string(redact("application/json", []byte(`{"id":"tr_1","destination":"acct_123","account":"acct_123"}`)))
	if !strings.Contains(out, `"account":"acct_123"`) {
		t.Fatalf("a Stripe account id is not a secret: %s", out)
	}
}

func TestKeyName(t *testing.T) {
	for in, want := range map[string]string{
		"account_number": "account_number", "external_account[account_number]": "account_number",
		"a[b][c]": "c", "plain": "plain", "odd[": "odd[",
	} {
		if got := keyName(in); got != want {
			t.Errorf("keyName(%q) = %q, want %q", in, got, want)
		}
	}
}
