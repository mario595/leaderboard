package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mario595/leaderboard/internal/store"
)

// EventInserter stores a webhook event. It is defined here, where it is
// consumed; *store.EventStore satisfies it.
type EventInserter interface {
	Insert(ctx context.Context, e store.Event) (bool, error)
}

// Handler serves Strava's webhook endpoint: the subscription handshake
// (Verify) and incoming events (Receive).
type Handler struct {
	events         EventInserter
	verifyToken    string
	subscriptionID int64
	wake           chan<- struct{}
	logger         *slog.Logger
}

// New returns a Handler. verifyToken is the secret given to Strava when the
// subscription is created. subscriptionID is the ID Strava returned for it;
// while it is 0, events are rejected. Each stored event is signalled on wake
// without blocking; wake may be nil while no worker listens.
func New(events EventInserter, verifyToken string, subID int64, wake chan<- struct{}, logger *slog.Logger) *Handler {
	return &Handler{
		events:         events,
		verifyToken:    verifyToken,
		subscriptionID: subID,
		wake:           wake,
		logger:         logger,
	}
}

// Verify answers the GET Strava sends while creating the subscription. When
// hub.mode is "subscribe" and hub.verify_token matches ours, it echoes
// hub.challenge as {"hub.challenge": "..."}; otherwise it answers 403 without
// the challenge, so only a caller who knows the token can complete it.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")
	if mode != "subscribe" || subtle.ConstantTimeCompare([]byte(token), []byte(h.verifyToken)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge})
}
