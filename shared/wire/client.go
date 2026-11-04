package wire

import (
	"net/http"
	"net/http/httptest"
	"time"
)

// Client returns an HTTP client whose requests are served by p directly, with
// no listener. A demo that makes its own vendor calls uses it, so every call is
// recorded and no port is opened that could clash with LocalStack's.
//
// The request's URL host is ignored: p sends the call to its own target.
func (p *Proxy) Client() *http.Client {
	return &http.Client{
		Transport: inProcess{h: p},
		Timeout:   90 * time.Second,
		// A vendor's redirect belongs to the caller, as with the network proxy.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type inProcess struct{ h http.Handler }

func (t inProcess) RoundTrip(r *http.Request) (*http.Response, error) {
	// A server always has a body to read, and a client request may have none.
	if r.Body == nil {
		r = r.Clone(r.Context())
		r.Body = http.NoBody
	}
	rec := httptest.NewRecorder()
	t.h.ServeHTTP(rec, r)
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}
