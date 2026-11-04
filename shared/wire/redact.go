package wire

import (
	"encoding/json"
	"net/url"
	"strings"
)

const redacted = "[redacted]"

// sensitiveKeys are matched case-insensitively against JSON keys and form
// fields. The inspector may be on a projector, so credentials never reach it,
// even the made-up ones an app emulator accepts.
var sensitiveKeys = []string{
	"secret", "client_secret", "access_token", "refresh_token", "id_token",
	"password", "api_key", "apikey", "authorization", "public_token",
}

func sensitive(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if k == s || strings.HasSuffix(k, "_"+s) {
			return true
		}
	}
	return false
}

// redact masks credential values in a JSON or form-encoded body and returns
// the body unchanged for any other content type or if it does not parse.
func redact(contentType string, body []byte) []byte {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			return body
		}
		out, err := json.Marshal(redactValue(v))
		if err != nil {
			return body
		}
		return out
	case strings.Contains(ct, "x-www-form-urlencoded"):
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return body
		}
		for k := range form {
			if sensitive(k) {
				form[k] = []string{redacted}
			}
		}
		return []byte(form.Encode())
	}
	return body
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if sensitive(k) {
				if _, isString := val.(string); isString {
					t[k] = redacted
					continue
				}
			}
			t[k] = redactValue(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	}
	return v
}
