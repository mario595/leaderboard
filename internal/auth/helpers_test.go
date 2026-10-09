package auth

import "testing"

func assertEqual[T comparable](t *testing.T, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("unexpected value %s: wanted %v, got %v", name, want, got)
	}
}
