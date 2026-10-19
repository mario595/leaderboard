package testutil

import (
	"testing"
	"time"
)

func AssertEqual[T comparable](t testing.TB, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("unexpected value %s: wanted %v, got %v", name, want, got)
	}
}

func AssertTime(t testing.TB, propertyName string, wanted, got time.Time) {
	t.Helper()
	if !wanted.Truncate(time.Second).Equal(got) {
		t.Errorf("unexpected value time %s: wanted %v, got %v", propertyName, wanted, got)
	}
}
