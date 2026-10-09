package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewState(t *testing.T) {
	t.Run("has the expected length", func(t *testing.T) {
		// 32 random bytes encode to 43 characters in unpadded base64.
		got := newState()
		if len(got) != 43 {
			t.Errorf("len(newState()) = %d, want 43", len(got))
		}
	})

	t.Run("is safe in a URL and a cookie", func(t *testing.T) {
		// Many samples, because a standard base64 encoding would only produce
		// '+', '/' or '=' for some values.
		for range 200 {
			got := newState()
			if strings.ContainsAny(got, "+/=") {
				t.Fatalf("newState() = %q, contains a character that is not URL-safe", got)
			}
		}
	})

	t.Run("decodes to 32 bytes", func(t *testing.T) {
		raw, err := base64.RawURLEncoding.DecodeString(newState())
		if err != nil {
			t.Fatalf("newState() is not valid unpadded base64url: %v", err)
		}
		if len(raw) != 32 {
			t.Errorf("decoded length = %d, want 32", len(raw))
		}
	})

	t.Run("is different every time", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 1000 {
			got := newState()
			if seen[got] {
				t.Fatalf("newState() returned %q twice", got)
			}
			seen[got] = true
		}
	})
}
