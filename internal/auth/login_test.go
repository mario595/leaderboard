package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/testutil"
)

// loginResult holds what the tests need from a response to Login.
type loginResult struct {
	status  int
	loc     *url.URL
	cookies []*http.Cookie
}

// login calls the Login handler and returns the parts of the response that the
// tests look at. It closes the response body itself.
func login(t *testing.T, h *Handler) loginResult {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/strava", nil)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	loc, err := res.Location()
	if err != nil {
		t.Fatalf("Location(): %v", err)
	}
	return loginResult{status: res.StatusCode, loc: loc, cookies: res.Cookies()}
}

// stateCookie returns the only cookie of the result, which must be oauth_state.
func (r loginResult) stateCookie(t *testing.T) *http.Cookie {
	t.Helper()

	if len(r.cookies) != 1 {
		t.Fatalf("got %d cookies, want exactly 1", len(r.cookies))
	}
	if r.cookies[0].Name != "oauth_state" {
		t.Fatalf("cookie name = %q, want %q", r.cookies[0].Name, "oauth_state")
	}
	return r.cookies[0]
}

func TestLogin_redirectsToStrava(t *testing.T) {
	res := login(t, newTestHandler(false))

	if res.status != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.status, http.StatusFound)
	}
	if res.loc.Host != "www.strava.com" || res.loc.Path != "/oauth/authorize" {
		t.Errorf("redirect to %s%s, want www.strava.com/oauth/authorize", res.loc.Host, res.loc.Path)
	}

	q := res.loc.Query()
	testutil.AssertEqual(t, "client_id", "test-id", q.Get("client_id"))
	testutil.AssertEqual(t, "response_type", "code", q.Get("response_type"))
	testutil.AssertEqual(t, "redirect_uri", "http://localhost:8080/auth/strava/callback", q.Get("redirect_uri"))

	scope := q.Get("scope")
	for _, want := range []string{"read", "activity:read"} {
		if !strings.Contains(scope, want) {
			t.Errorf("scope = %q, want it to contain %q", scope, want)
		}
	}

	if q.Get("state") == "" {
		t.Error("state is empty")
	}
}

func TestLogin_stateCookie(t *testing.T) {
	res := login(t, newTestHandler(false))
	c := res.stateCookie(t)

	// The cookie and the URL carry the same state: this is what the callback
	// compares to prove that the browser started the flow.
	testutil.AssertEqual(t, "cookie value", res.loc.Query().Get("state"), c.Value)

	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if c.Secure {
		t.Error("cookie is Secure, want not Secure when secureCookies is false")
	}
	testutil.AssertEqual(t, "cookie path", "/auth/strava", c.Path)
	testutil.AssertEqual(t, "cookie SameSite", http.SameSiteLaxMode, c.SameSite)
	testutil.AssertEqual(t, "cookie MaxAge", 600, c.MaxAge)
}

func TestLogin_secureCookie(t *testing.T) {
	res := login(t, newTestHandler(true))

	if !res.stateCookie(t).Secure {
		t.Error("cookie is not Secure, want Secure when secureCookies is true")
	}
}

func TestLogin_newStateEachTime(t *testing.T) {
	h := newTestHandler(false)

	first := login(t, h)
	second := login(t, h)

	if first.loc.Query().Get("state") == second.loc.Query().Get("state") {
		t.Error("two logins produced the same state")
	}
}
