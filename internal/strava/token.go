package strava

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// savingTokenSource wraps the library's refreshing TokenSource and saves every
// new token it hands out, so a refresh survives the process. It saves each new
// access token exactly once, even when several requests share the client: mu
// serialises the check-and-save, and last remembers what is already stored.
type savingTokenSource struct {
	src    oauth2.TokenSource
	userID int64
	store  TokenStore

	mu   sync.Mutex
	last string // access token last known to be stored
}

// Token returns the current token, refreshing it through src when it has
// expired. A new token is saved before it is returned; if the save fails the
// token is not used, and the next call tries to save it again.
func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err != nil {
		return nil, fmt.Errorf("retrieve token for user %d: %w", s.userID, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.AccessToken == s.last {
		return t, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = s.store.UpdateTokens(ctx, s.userID, t.AccessToken, t.RefreshToken, t.Expiry)
	if err != nil {
		return nil, fmt.Errorf("save refreshed token for user %d: %w", s.userID, err)
	}
	s.last = t.AccessToken
	return t, nil
}
