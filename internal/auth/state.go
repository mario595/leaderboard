package auth

import (
	"crypto/rand"
	"encoding/base64"
)

// newState returns an unguessable random value for the OAuth state parameter.
// It protects the login against cross-site request forgery.
func newState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
