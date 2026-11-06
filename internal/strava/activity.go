package strava

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Activity is a Strava activity with the fields the app uses, in plain Go
// types. It carries no user: the caller knows whose client fetched it.
type Activity struct {
	// ID is the Strava activity ID.
	ID int64
	// SportType is the Strava sport_type, for example "GravelRide".
	SportType string
	// StartLocal is the athlete's local wall-clock start time, stored as UTC
	// (Strava start_date_local). Its date is the day the activity scores on.
	StartLocal time.Time
	// MovingTime is the Strava moving_time.
	MovingTime time.Duration
	// Distance is the distance in metres.
	Distance float64
	// WorkoutType is the Strava workout_type, 0 when Strava sends null.
	WorkoutType int
	// Description is the activity description, "" when Strava sends null.
	Description string
}

// activityDTO is the part of Strava's DetailedActivity JSON the app uses.
// Unknown fields are ignored. A JSON null leaves a field at its zero value,
// which is the mapping we want for workout_type and description.
type activityDTO struct {
	ID             int64   `json:"id"`
	SportType      string  `json:"sport_type"`
	StartDateLocal string  `json:"start_date_local"`
	MovingTime     int     `json:"moving_time"`
	Distance       float64 `json:"distance"`
	WorkoutType    int     `json:"workout_type"`
	Description    string  `json:"description"`
}

// toActivity maps the DTO to an Activity. It converts types but does not
// validate: whether the activity counts is decided by scoring. It fails only
// if start_date_local cannot be parsed.
func (d activityDTO) toActivity() (Activity, error) {
	startDate, err := time.Parse(time.RFC3339, d.StartDateLocal)
	if err != nil {
		return Activity{}, fmt.Errorf("parse start date %q: %w", d.StartDateLocal, err)
	}

	return Activity{
		ID:          d.ID,
		SportType:   d.SportType,
		StartLocal:  startDate,
		MovingTime:  time.Duration(d.MovingTime) * time.Second,
		Distance:    d.Distance,
		WorkoutType: d.WorkoutType,
		Description: d.Description,
	}, nil
}

// Activity fetches the activity with the given Strava ID. Errors wrap
// ErrNotFound (deleted or private), ErrUnauthorized (access revoked) or
// *RateLimitError (quota used up) when those apply.
func (c *Client) Activity(ctx context.Context, id int64) (Activity, error) {
	path := fmt.Sprintf("/activities/%d", id)
	var dto activityDTO
	err := c.get(ctx, path, &dto)
	if err != nil {
		return Activity{}, fmt.Errorf("fetch activity %d: %w", id, err)
	}
	activity, err := dto.toActivity()
	if err != nil {
		return Activity{}, fmt.Errorf("fetch activity %d: %w", id, err)
	}
	return activity, nil
}

// ListActivities returns one page (numbered from 1) of the user's activities
// that started after after and before before. Strava compares both with the
// UTC start time, not the local one, so a caller filtering by local day must
// pad the range. A page past the last one is empty. The activities are
// summaries: Description is always "". Errors are typed as in Activity.
func (c *Client) ListActivities(ctx context.Context, after, before time.Time, page, perPage int) ([]Activity, error) {
	var dtos []activityDTO
	q := url.Values{
		"after":    {strconv.FormatInt(after.Unix(), 10)},
		"before":   {strconv.FormatInt(before.Unix(), 10)},
		"page":     {strconv.Itoa(page)},
		"per_page": {strconv.Itoa(perPage)},
	}
	if err := c.get(ctx, "/athlete/activities?"+q.Encode(), &dtos); err != nil {
		return nil, fmt.Errorf("list activities page %d: %w", page, err)
	}

	result := make([]Activity, 0, len(dtos))
	for _, dto := range dtos {
		a, err := dto.toActivity()
		if err != nil {
			return nil, fmt.Errorf("parsing activity dto %d: %w", dto.ID, err)
		}
		result = append(result, a)
	}
	return result, nil
}
