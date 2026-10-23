package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"

	"golang.org/x/oauth2"
)

// EventQueue is the webhook event queue; *store.EventStore satisfies it.
type EventQueue interface {
	NextDue(ctx context.Context) (store.Event, bool, error)
	MarkDone(ctx context.Context, id int64) error
	MarkRetry(ctx context.Context, id int64, errMsg string, next time.Time) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}

// Users finds the user owning an event and deletes athletes who
// deauthorize; *store.UserStore satisfies it.
type Users interface {
	GetByStravaID(ctx context.Context, stravaID int64) (store.User, error)
	DeleteByStravaID(ctx context.Context, stravaID int64) (bool, error)
}

// ActivitySaver writes activities and recomputes their days;
// *activity.Service satisfies it.
type ActivitySaver interface {
	Save(ctx context.Context, a store.Activity) error
	Delete(ctx context.Context, stravaID int64) error
}

// ActivityFetcher fetches one activity from Strava; *strava.Client
// satisfies it.
type ActivityFetcher interface {
	Activity(ctx context.Context, id int64) (strava.Activity, error)
}

// ClientFunc builds a Strava client acting for the user with the given
// internal ID. It is a function, not an interface, because
// (*strava.API).ForUser returns *strava.Client: Go method signatures must
// match exactly, so it could not satisfy a method returning ActivityFetcher.
type ClientFunc func(userID int64, tok *oauth2.Token) ActivityFetcher

// Worker processes webhook events in the background, one at a time, oldest
// first. For an activity event it asks Strava for the activity's current
// state and saves or deletes it, whatever the event's aspect type.
type Worker struct {
	events     EventQueue
	users      Users
	activities ActivitySaver
	newClient  ClientFunc
	wake       <-chan struct{}
	logger     *slog.Logger

	now          func() time.Time
	pollInterval time.Duration
}

// New returns a Worker. wake signals new events; the worker also polls every
// minute for retries that become due.
func New(events EventQueue, users Users, activities ActivitySaver, newClient ClientFunc, wake <-chan struct{}, logger *slog.Logger) *Worker {
	return &Worker{
		events:       events,
		users:        users,
		activities:   activities,
		newClient:    newClient,
		wake:         wake,
		logger:       logger,
		now:          time.Now,
		pollInterval: time.Minute,
	}
}

// process handles one event: an activity event brings the activity in line
// with Strava (ignored when its owner is not a user); an athlete event
// deletes the athlete when they deauthorized; anything else is ignored (nil).
// The returned error decides how handle marks the event.
func (w *Worker) process(ctx context.Context, e store.Event) error {
	w.logger.Info("processing event", "id", e.ID, "object_type", e.ObjectType, "aspect_type", e.AspectType, "owner_id", e.OwnerID, "object_id", e.ObjectID)

	switch e.ObjectType {
	case "athlete":
		return w.processAthleteEvent(ctx, e)
	case "activity":
		return w.processActivityEvent(ctx, e.ObjectID, e.OwnerID)
	default:
		return nil
	}
}

// processAthleteEvent deletes everything stored for an athlete who
// deauthorized the app (updates {"authorized":"false"}), user or not. Other
// athlete updates are ignored, and so are updates that cannot be decoded:
// retrying would not fix them, so they are logged instead.
func (w *Worker) processAthleteEvent(ctx context.Context, e store.Event) error {
	var u struct {
		Authorized string `json:"authorized"`
	}
	if err := json.Unmarshal([]byte(e.Updates), &u); err != nil {
		w.logger.Warn("cannot decode athlete updates", "event_id", e.ID, "updates", e.Updates, "error", err)
		return nil
	}
	if u.Authorized == "false" {
		deleted, err := w.users.DeleteByStravaID(ctx, e.ObjectID)
		if err != nil {
			return fmt.Errorf("delete athlete %d: %w", e.ObjectID, err)
		}
		w.logger.Info("athlete deauthorized", "athlete_id", e.ObjectID, "deleted", deleted)
	}
	return nil
}

// processActivityEvent brings one activity in line with Strava, using its
// owner's token. Events of athletes who are not users are ignored.
func (w *Worker) processActivityEvent(ctx context.Context, activityID int64, athleteID int64) error {
	u, err := w.users.GetByStravaID(ctx, athleteID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get by strava ID %d: %w", athleteID, err)
	}
	tok := &oauth2.Token{
		AccessToken:  u.AccessToken,
		RefreshToken: u.RefreshToken,
		Expiry:       u.TokenExpiresAt,
	}
	client := w.newClient(u.ID, tok)
	stravaActivity, err := client.Activity(ctx, activityID)
	if errors.Is(err, strava.ErrNotFound) {
		w.logger.Info("strava activity not found, deleting", "activity_id", activityID)
		if err := w.activities.Delete(ctx, activityID); err != nil {
			return fmt.Errorf("delete activity %d for athlete %d: %w", activityID, u.StravaID, err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("fetch activity %d for athlete %d: %w", activityID, u.StravaID, err)
	}

	w.logger.Info("strava activity found, updating", "activity_id", activityID)
	err = w.activities.Save(ctx, toStoreActivity(u.ID, stravaActivity))
	if err != nil {
		return fmt.Errorf("save activity %d for athlete %d: %w", activityID, u.StravaID, err)
	}
	return nil
}

// toStoreActivity maps a Strava activity to the stored form for the user with
// the given internal ID.
func toStoreActivity(userID int64, a strava.Activity) store.Activity {
	return store.Activity{
		StravaID:    a.ID,
		UserID:      userID,
		SportType:   a.SportType,
		StartLocal:  a.StartLocal,
		MovingTime:  a.MovingTime,
		Distance:    a.Distance,
		WorkoutType: a.WorkoutType,
		Description: a.Description,
	}
}
