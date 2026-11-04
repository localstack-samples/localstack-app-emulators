package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/localstack-samples/localstack-app-emulators/shared/vendorcfg"
)

// maxRecorded caps how much of a body is kept in an Event. The call itself is
// always forwarded in full.
const maxRecorded = 64 << 10

// LocalStack's gateway serves each app emulator on its own host name. Dialling
// the gateway's address and setting the Host header avoids DNS, so the demo
// still resolves with no internet (localhost.localstack.cloud is a public DNS
// record that points at 127.0.0.1).
const (
	localStackEdge     = "http://127.0.0.1:4566"
	localStackHostFmt  = "%s.localhost.localstack.cloud:4566"
	hopByHopConnection = "Connection"
)

// Who hosts an app emulator.
const (
	SourceLocalStack = "localstack" // LocalStack's gateway serves it
	SourceLocal      = "local"      // you run it yourself, such as a binary built from WonderTwin-AI/wondertwin
)

// Target is where one vendor's calls go.
type Target struct {
	Vendor   string // lane name in the inspector, such as "stripe"
	Source   string // SourceLocalStack or SourceLocal
	Upstream string // base URL dialled, such as http://127.0.0.1:4566
	Host     string // Host header sent upstream; empty keeps the proxy's own
}

// LocalStack is the Target for an app emulator served by LocalStack's gateway
// on the default port.
func LocalStack(vendor string) Target {
	return Target{
		Vendor:   vendor,
		Source:   SourceLocalStack,
		Upstream: localStackEdge,
		Host:     fmt.Sprintf(localStackHostFmt, vendor),
	}
}

// Direct is the Target for an app emulator you run yourself at baseURL. The
// Host header is left alone.
func Direct(vendor, baseURL string) Target {
	return Target{Vendor: vendor, Source: SourceLocal, Upstream: strings.TrimRight(baseURL, "/")}
}

// TargetFromEnv chooses where a vendor's app emulator is. If <VENDOR>_EMULATOR_URL
// is set, you are running it yourself there; otherwise LocalStack hosts it.
//
// This is how a vendor moves between the two: Plaid and Xero run locally until
// LocalStack includes them, and then unsetting one variable switches each one,
// with no code change.
func TargetFromEnv(getenv func(string) string, vendor string) Target {
	if u := getenv(vendorcfg.EmulatorEnvName(vendor)); u != "" {
		return Direct(vendor, u)
	}
	return LocalStack(vendor)
}

// Proxy forwards each request to a Target and records it. The whole request
// and response are buffered, so it suits API calls and not streaming.
type Proxy struct {
	target Target
	rec    *Recorder
	client *http.Client
}

// NewProxy returns a handler that forwards to t and records into rec.
func NewProxy(t Target, rec *Recorder) *Proxy {
	return &Proxy{
		target: t,
		rec:    rec,
		client: &http.Client{
			Timeout: 60 * time.Second,
			// A vendor's redirect belongs to the SDK, not to the proxy.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ev := Event{Time: start, Vendor: p.target.Vendor, Source: p.target.Source, Method: r.Method, Path: r.URL.RequestURI()}

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		p.fail(w, ev, start, http.StatusBadRequest, fmt.Errorf("read request: %w", err))
		return
	}
	ev.Request = capture(r.Header.Get("Content-Type"), reqBody)

	out, err := http.NewRequestWithContext(r.Context(), r.Method, p.target.Upstream+r.URL.RequestURI(), bytes.NewReader(reqBody))
	if err != nil {
		p.fail(w, ev, start, http.StatusBadGateway, err)
		return
	}
	out.Header = r.Header.Clone()
	out.Header.Del(hopByHopConnection)
	if p.target.Host != "" {
		out.Host = p.target.Host
	}

	resp, err := p.client.Do(out)
	if err != nil {
		p.fail(w, ev, start, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.fail(w, ev, start, http.StatusBadGateway, fmt.Errorf("read response: %w", err))
		return
	}

	for k, vs := range resp.Header {
		if k == hopByHopConnection || k == "Content-Length" {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	ev.Status = resp.StatusCode
	ev.Duration = time.Since(start)
	ev.Response = capture(resp.Header.Get("Content-Type"), respBody)
	p.rec.Record(ev)
}

// fail answers the caller with a JSON error and records the failed call.
func (p *Proxy) fail(w http.ResponseWriter, ev Event, start time.Time, status int, err error) {
	ev.Status = status
	ev.Duration = time.Since(start)
	ev.Error = err.Error()
	p.rec.Record(ev)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":  "wire: " + strings.ToLower(http.StatusText(status)),
		"vendor": p.target.Vendor,
		"detail": err.Error(),
	})
}

// capture redacts credentials and keeps at most maxRecorded bytes of body.
func capture(contentType string, body []byte) Body {
	b := Body{Size: len(body)}
	body = redact(contentType, body)
	if len(body) > maxRecorded {
		body = body[:maxRecorded]
		b.Truncated = true
	}
	b.Content = string(body)
	return b
}
