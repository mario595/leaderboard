package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	normalRouter := NewRouter(logger)

	panicRouter := chi.NewRouter()
	panicRouter.Use(middleware.RequestID)
	panicRouter.Use(requestLogger(logger))
	panicRouter.Use(middleware.Recoverer)

	panicRouter.Get("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })

	tests := []struct {
		name   string
		router http.Handler
		msg    string
		method string
		path   string
		status float64
	}{{
		name:   "Normal_request",
		router: normalRouter,
		msg:    "request",
		method: http.MethodGet,
		path:   "/healthz",
		status: http.StatusOK,
	},
		{
			name:   "Panic_request",
			router: panicRouter,
			msg:    "request",
			method: http.MethodGet,
			path:   "/boom",
			status: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			res := httptest.NewRecorder()
			tc.router.ServeHTTP(res, req)
			out := decodeLogLine(t, &buf)

			wanted := "request"
			got := out["msg"]
			if got != wanted {
				t.Errorf("incorrect msg. wanted %v, got: %v", wanted, got)
			}

			wanted = tc.method
			got = out["method"]

			if got != wanted {
				t.Errorf("incorrect method. wanted %v, got: %v", wanted, got)
			}

			wanted = tc.path
			got = out["path"]

			if got != wanted {
				t.Errorf("incorrect path. wanted %v, got: %v", wanted, got)
			}

			if out["request_id"] == nil {
				t.Errorf("Missing request_id")
			}

			wantedStatus := float64(tc.status)
			gotStatus := out["status"]

			if gotStatus != wantedStatus {
				t.Errorf("incorrect status. wanted %v, got: %v", wantedStatus, gotStatus)
			}
		})
	}
}

func decodeLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Error unmarshalling output, %v", err)
	}
	return out
}
