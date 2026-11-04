package world

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type call struct {
	Method, Path string
	Body         string
}

// fake is an app emulator that answers fixed JSON for the routes it is given
// and records every call. Health, reset and clock routes always work.
type fake struct {
	srv    *httptest.Server
	mu     sync.Mutex
	calls  []call
	routes map[string]string // "METHOD /path" to the JSON it answers
}

func newFake(t *testing.T, routes map[string]string) *fake {
	f := &fake{routes: routes}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, call{r.Method, r.URL.Path, string(b)})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/health":
			w.Write([]byte(`{"status":"ok"}`))
			return
		case "/admin/reset", "/admin/time/advance":
			w.Write([]byte(`{}`))
			return
		}
		if body, ok := f.routes[r.Method+" "+r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) to(method, path string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

func (f *fake) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	for i, c := range f.calls {
		out[i] = c.Method + " " + c.Path
	}
	return out
}
