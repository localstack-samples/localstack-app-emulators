package seed

import (
	"strings"
	"testing"
)

func TestErrorUsesTheProxysDetailAndKeepsAVendorsOwnBody(t *testing.T) {
	proxy := &Error{Vendor: "stripe", Method: "GET", Path: "/v1/balance", Status: 503,
		Body: `{"detail":"dry run: this GET /v1/balance was not sent","error":"wire: service unavailable","vendor":"stripe"}`}
	if got := proxy.Error(); got != "dry run: this GET /v1/balance was not sent" {
		t.Errorf("proxy error = %q", got)
	}

	vendor := &Error{Vendor: "stripe", Method: "GET", Path: "/v1/balance", Status: 401, Body: `{"error":{"message":"No API key provided"}}`}
	if got := vendor.Error(); !strings.HasPrefix(got, "stripe GET /v1/balance: HTTP 401: ") || !strings.Contains(got, "No API key provided") {
		t.Errorf("vendor error = %q", got)
	}

	plain := &Error{Vendor: "xero", Method: "GET", Path: "/x", Status: 500, Body: "boom"}
	if got := plain.Error(); got != "xero GET /x: HTTP 500: boom" {
		t.Errorf("plain error = %q", got)
	}
}
