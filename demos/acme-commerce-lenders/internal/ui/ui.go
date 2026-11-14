// Package ui serves the compiled web UI from inside the binary. `make web`
// writes the build into assets/; until then the committed placeholder is
// served, so the binary always builds.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:assets
var assets embed.FS

// Handler serves the UI from the embedded assets.
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err) // the embedded directory is part of the source tree
	}
	return handler(sub)
}

// Built reports whether the binary carries the built web UI rather than the
// placeholder page.
func Built() bool {
	sub, err := fs.Sub(assets, "assets")
	return err == nil && exists(sub, "index.html")
}

// handler serves fsys as a single-page app:
//   - a file that exists is served as it is;
//   - a path with no file extension that does not exist, such as /inspector, is
//     a route inside the app, so it gets index.html and the app does the routing;
//   - a missing file with an extension, such as /assets/x.js, is a 404;
//   - with no build yet, / gets the placeholder page.
func handler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if !exists(fsys, name) {
			switch {
			case name == "index.html":
				name = "placeholder.html" // there is no build yet
			case path.Ext(name) != "":
				http.NotFound(w, r)
				return
			default:
				name = "index.html" // a route inside the app
				if !exists(fsys, name) {
					name = "placeholder.html"
				}
			}
		}

		// Built assets have a hash in their name, so they never change. The page
		// that names them must always be fetched fresh.
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		http.ServeFileFS(w, r, fsys, name)
	})
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
