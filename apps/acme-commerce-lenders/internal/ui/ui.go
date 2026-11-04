// Package ui serves the compiled web UI from inside the binary. The web build
// writes its output into assets/ (see the Makefile); until then the committed
// placeholder is served, so the binary always builds.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:assets
var assets embed.FS

// Handler serves the UI. It serves index.html when the web build has produced
// one, and the placeholder page otherwise.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // the embedded directory is part of the source tree
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if _, err := fs.Stat(sub, "index.html"); err != nil {
				r.URL.Path = "/placeholder.html"
			}
		}
		files.ServeHTTP(w, r)
	})
}
