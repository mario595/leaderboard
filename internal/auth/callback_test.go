package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCallback(t *testing.T) {
	testCases := []struct {
		name              string
		cookie            string
		urlValues         url.Values
		expectedStatus    int
		wantCookieCleared bool
	}{
		{
			name:           "user_declined",
			urlValues:      url.Values{"error": {"access_denied"}},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "no_cookie",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "cookie_differs",
			cookie: "other",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "empty_state",
			cookie: "abc",
			urlValues: url.Values{
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "missing_code",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "scope_read_only",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read"},
			},
			expectedStatus:    http.StatusForbidden,
			wantCookieCleared: true,
		},
		{
			name:   "comma_scope_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read,activity:read"},
			},
			expectedStatus:    http.StatusNoContent,
			wantCookieCleared: true,
		},
		{
			name:   "space_scope_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read activity:read"},
			},
			expectedStatus:    http.StatusNoContent,
			wantCookieCleared: true,
		},
		{
			name:   "read_all_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read,activity:read_all"},
			},
			expectedStatus:    http.StatusNoContent,
			wantCookieCleared: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(false)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/strava/callback?"+tc.urlValues.Encode(), nil)

			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "oauth_state", Value: tc.cookie})
			}

			rec := httptest.NewRecorder()

			h.Callback(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.expectedStatus)
			}

			if tc.wantCookieCleared {
				res := rec.Result()
				defer func() { _ = res.Body.Close() }()
				var found *http.Cookie
				for _, c := range res.Cookies() {
					if c.Name == "oauth_state" {
						found = c
					}
				}

				if found == nil {
					t.Fatalf("no oauth_state cookie in the response")
				}
				cookieMaxAge := found.MaxAge
				if cookieMaxAge >= 0 {
					t.Errorf("Cookie oauth_state MaxAge %d, expected -1", cookieMaxAge)
				}
			}

		})
	}

}
