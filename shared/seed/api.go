package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/wire"
)

// API makes calls to one app emulator. It reaches it the way wire.Proxy does,
// by dialling the target's upstream and setting its Host header, so the same
// seeders work against LocalStack's gateway and against a self-run app
// emulator.
//
// HC is how the calls are sent. A demo can pass a client that routes through
// the wire proxy, so the inspector can show seed calls as setup.
type API struct {
	HC     *http.Client
	Target wire.Target
}

// Error is a non-2xx answer from an app emulator.
type Error struct {
	Vendor string
	Method string
	Path   string
	Status int
	Body   string // the start of the response body
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s %s %s: HTTP %d: %s", e.Vendor, e.Method, e.Path, e.Status, e.Body)
}

func (a API) client() *http.Client {
	if a.HC != nil {
		return a.HC
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// Do sends a request and decodes a JSON answer into out, if out is not nil.
func (a API) Do(ctx context.Context, method, path string, headers map[string]string, contentType string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, a.Target.Upstream+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if a.Target.Host != "" {
		req.Host = a.Target.Host
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client().Do(req)
	if err != nil {
		return fmt.Errorf("%s %s %s: %w", a.Target.Vendor, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s %s: read answer: %w", a.Target.Vendor, method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{Vendor: a.Target.Vendor, Method: method, Path: path, Status: resp.StatusCode, Body: excerpt(raw)}
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s %s %s: decode answer: %w", a.Target.Vendor, method, path, err)
	}
	return nil
}

// JSON sends in as a JSON body.
func (a API) JSON(ctx context.Context, method, path string, headers map[string]string, in, out any) error {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	return a.Do(ctx, method, path, headers, "application/json", body, out)
}

// Form sends form as an application/x-www-form-urlencoded body.
func (a API) Form(ctx context.Context, method, path string, headers map[string]string, form url.Values, out any) error {
	return a.Do(ctx, method, path, headers, "application/x-www-form-urlencoded", []byte(form.Encode()), out)
}

func excerpt(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
