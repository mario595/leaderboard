package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

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

// eventDTO is the JSON body of a Strava webhook event. Updates is kept as raw
// JSON: its values are loosely typed and only athlete events use them (#15).
type eventDTO struct {
	ObjectType     string          `json:"object_type"`
	ObjectID       int64           `json:"object_id"`
	AspectType     string          `json:"aspect_type"`
	OwnerID        int64           `json:"owner_id"`
	EventTime      int64           `json:"event_time"`
	SubscriptionID int64           `json:"subscription_id"`
	Updates        json.RawMessage `json:"updates"`
}

// Receive accepts an event Strava POSTs to the callback. It answers 403 when
// the event is not for our subscription (or no subscription is configured),
// 400 when the body is too large, malformed or has unknown types or
// non-positive IDs, and 500 when the event cannot be stored, so Strava
// retries. 200 means the event is stored, now or by an earlier delivery.
// A newly stored event wakes the worker without blocking.
func (h *Handler) Receive(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var dto eventDTO
	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		h.logger.Warn("cannot decode event", "error", err)
		http.Error(w, "cannot decode event", http.StatusBadRequest)
		return
	}
	if h.subscriptionID == 0 || dto.SubscriptionID != h.subscriptionID {
		h.logger.Warn("wrong subscriptionID", "subscriptionID", dto.SubscriptionID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if !slices.Contains([]string{"create", "update", "delete"}, dto.AspectType) {
		h.logger.Warn("unknown aspect_type", "aspect_type", dto.AspectType)
		http.Error(w, "unknown aspect_type", http.StatusBadRequest)
		return
	}

	if !slices.Contains([]string{"activity", "athlete"}, dto.ObjectType) {
		h.logger.Warn("unknown object_type", "object_type", dto.ObjectType)
		http.Error(w, "unknown object_type", http.StatusBadRequest)
		return
	}

	if dto.ObjectID <= 0 || dto.OwnerID <= 0 || dto.EventTime <= 0 {
		h.logger.Warn("malformed request", "object_id", dto.ObjectID, "owner_id", dto.OwnerID, "event_time", dto.EventTime)
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}

	updates := string(dto.Updates)
	if len(dto.Updates) == 0 || string(dto.Updates) == "null" {
		updates = "{}"
	}

	inserted, err := h.events.Insert(r.Context(), store.Event{
		ObjectType: dto.ObjectType,
		ObjectID:   dto.ObjectID,
		AspectType: dto.AspectType,
		OwnerID:    dto.OwnerID,
		EventTime:  time.Unix(dto.EventTime, 0),
		Updates:    updates,
	})
	if err != nil {
		h.logger.Error("inserting event", "error", err)
		http.Error(w, "could not store event", http.StatusInternalServerError)
		return
	}
	if !inserted {
		h.logger.Debug("duplicate event ignored", "object_type", dto.ObjectType, "object_id", dto.ObjectID, "aspect_type", dto.AspectType, "owner_id", dto.OwnerID)
		return
	}

	h.logger.Info("inserted event", "object_type", dto.ObjectType, "object_id", dto.ObjectID, "aspect_type", dto.AspectType, "owner_id", dto.OwnerID)
	select {
	case h.wake <- struct{}{}:
	default:
	}
}
