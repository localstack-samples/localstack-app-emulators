package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

var built = fstest.MapFS{
	"index.html":             {Data: []byte(`<div id="root"></div>`)},
	"placeholder.html":       {Data: []byte("placeholder")},
	"assets/index-abc123.js": {Data: []byte("console.log(1)")},
}

func TestHandlerServesPlaceholderWhenNoBuild(t *testing.T) {
	rec := get(Handler(), "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ACME Commerce Lenders") {
		t.Fatalf("the embedded placeholder should be served: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerServesTheBuiltApp(t *testing.T) {
	h := handler(built)
	if rec := get(h, "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Fatalf("/ should be the app, not the placeholder: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get(h, "/index.html"); rec.Code != http.StatusMovedPermanently && rec.Code != 200 {
		t.Fatalf("/index.html: %d", rec.Code)
	}
}

// A client-side route has no file. Loading it directly must still give the app.
func TestAClientSideRouteGetsTheApp(t *testing.T) {
	h := handler(built)
	for _, p := range []string{"/inspector", "/inspector/events", "/some/deep/route"} {
		if rec := get(h, p); rec.Code != 200 || !strings.Contains(rec.Body.String(), `id="root"`) {
			t.Errorf("%s should serve the app: %d %s", p, rec.Code, rec.Body.String())
		}
	}
}

func TestAMissingFileIsA404NotTheApp(t *testing.T) {
	h := handler(built)
	for _, p := range []string{"/assets/missing.js", "/favicon.ico", "/robots.txt"} {
		if rec := get(h, p); rec.Code != http.StatusNotFound {
			t.Errorf("%s should be a 404, got %d (the app must not answer for a missing file)", p, rec.Code)
		}
	}
}

func TestHashedAssetsAreCachedForeverAndThePageIsNot(t *testing.T) {
	h := handler(built)
	if got := get(h, "/assets/index-abc123.js").Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("a hashed asset never changes, so it should be immutable: %q", got)
	}
	if got := get(h, "/").Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("the page that names the assets must be fetched fresh: %q", got)
	}
}

func TestADotDotPathCannotEscapeTheAssets(t *testing.T) {
	h := handler(built)
	rec := get(h, "/../../etc/passwd")
	if strings.Contains(rec.Body.String(), "root:") {
		t.Fatal("a path must not leave the embedded assets")
	}
}

func TestARouteWithNoBuildGetsThePlaceholder(t *testing.T) {
	h := handler(fstest.MapFS{"placeholder.html": {Data: []byte("placeholder")}})
	for _, p := range []string{"/", "/inspector"} {
		if rec := get(h, p); rec.Code != 200 || rec.Body.String() != "placeholder" {
			t.Errorf("%s with no build: %d %q", p, rec.Code, rec.Body.String())
		}
	}
}
