package spa

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist is the frontend build (web/, built into internal/spa/dist). all:
// includes the .gitkeep placeholder, which keeps the pattern matching before
// the first build.
//
//go:embed all:dist
var dist embed.FS

// Handler serves the embedded frontend build. See newHandler.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // only an invalid constant name can fail
	}
	return newHandler(sub)
}

// newHandler serves the files in fsys, falling back to index.html for page
// routes so React Router can pick the page:
//   - an existing file is served as is; files under assets/ have a content
//     hash in their names and are cached for good, anything else is
//     revalidated;
//   - a missing path with an extension is a 404 (an old bundle asked for by a
//     stale tab must not get HTML);
//   - any other path is index.html, never cached, or a 503 when the frontend
//     has not been built.
func newHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "/" gives "", which fs.Stat rejects: it falls through to index.html.
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		info, err := fs.Stat(fsys, name)
		if err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			http.ServeFileFS(w, r, fsys, name)
			return
		}
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}

		if _, err := fs.Stat(fsys, "index.html"); err != nil {
			http.Error(w, "frontend not built: run make web-build", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	})
}
