package spa

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mario595/leaderboard/internal/testutil"
)

const (
	noCache   = "no-cache"
	immutable = "public, max-age=31536000, immutable"
)

// built is what `npm run build` writes: the page, a public file, and the
// hashed bundle under assets/.
var built = fstest.MapFS{
	"index.html":           {Data: []byte("<!doctype html><title>app</title>")},
	"favicon.svg":          {Data: []byte("<svg></svg>")},
	"assets/index-3f9a.js": {Data: []byte("console.log('app')")},
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string // "" skips the check
		wantType    string // prefix of Content-Type; "" skips the check
		wantCaching string // "" skips the check
	}{
		{name: "root is the app", path: "/", wantStatus: http.StatusOK, wantBody: "<title>app</title>", wantType: "text/html", wantCaching: noCache},
		{name: "a client route is the app", path: "/activities", wantStatus: http.StatusOK, wantBody: "<title>app</title>", wantType: "text/html", wantCaching: noCache},
		{name: "a nested client route is the app", path: "/a/b/c/", wantStatus: http.StatusOK, wantBody: "<title>app</title>", wantType: "text/html", wantCaching: noCache},
		{name: "a directory is the app, never a listing", path: "/assets", wantStatus: http.StatusOK, wantBody: "<title>app</title>", wantCaching: noCache},
		{name: "hashed asset is cached for good", path: "/assets/index-3f9a.js", wantStatus: http.StatusOK, wantBody: "console.log('app')", wantType: "text/javascript", wantCaching: immutable},
		{name: "public file is revalidated", path: "/favicon.svg", wantStatus: http.StatusOK, wantBody: "<svg></svg>", wantType: "image/svg+xml", wantCaching: noCache},
		{name: "missing asset is 404, not the app", path: "/assets/old-1234.js", wantStatus: http.StatusNotFound},
		{name: "missing file at the root is 404", path: "/robots.txt", wantStatus: http.StatusNotFound},
		{name: "HEAD has headers and no body", method: http.MethodHead, path: "/activities", wantStatus: http.StatusOK, wantType: "text/html", wantCaching: noCache},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			rec := httptest.NewRecorder()
			newHandler(built).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, tt.path, nil))

			testutil.AssertEqual(t, "status", tt.wantStatus, rec.Code)
			body := rec.Body.String()
			if tt.wantBody != "" && !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", body, tt.wantBody)
			}
			if tt.wantStatus == http.StatusNotFound && strings.Contains(body, "<title>app</title>") {
				t.Errorf("404 body is the app: %q", body)
			}
			if method == http.MethodHead && body != "" {
				t.Errorf("HEAD body = %q, want none", body)
			}
			if got := rec.Header().Get("Content-Type"); tt.wantType != "" && !strings.HasPrefix(got, tt.wantType) {
				t.Errorf("Content-Type = %q, want prefix %q", got, tt.wantType)
			}
			if tt.wantCaching != "" {
				testutil.AssertEqual(t, "Cache-Control", tt.wantCaching, rec.Header().Get("Cache-Control"))
			}
		})
	}
}

// Before `make web-build` the embedded folder holds only the placeholder:
// pages answer 503 with what to do, and files are still plain 404s.
func TestHandlerNotBuilt(t *testing.T) {
	notBuilt := fstest.MapFS{".gitkeep": {Data: nil}}

	rec := httptest.NewRecorder()
	newHandler(notBuilt).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activities", nil))
	testutil.AssertEqual(t, "page status", http.StatusServiceUnavailable, rec.Code)
	if !strings.Contains(rec.Body.String(), "make web-build") {
		t.Errorf("body = %q, want it to say how to build the frontend", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	newHandler(notBuilt).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/index-3f9a.js", nil))
	testutil.AssertEqual(t, "asset status", http.StatusNotFound, rec.Code)
}

// Handler must serve the embedded build from its root (fs.Sub), whatever is
// built: the placeholder is there in every checkout, built or not.
func TestHandlerEmbedsDist(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.gitkeep", nil))
	testutil.AssertEqual(t, "status of the embedded placeholder", http.StatusOK, rec.Code)
}
