// Package worker processes Strava webhook events in the background.
//
// One goroutine runs Worker.Run. It takes due events from the webhook_events
// table oldest first, so pending events survive restarts; the webhook
// handler wakes it when a new event is stored, and it also polls every
// minute for retries that become due.
//
// For an activity event, whatever its aspect type, the worker asks Strava for
// the activity with the owner's token: if Strava has it, it is saved and its
// day recomputed; if Strava answers not found, it is deleted. Strava's
// current state wins, which also covers forged or out-of-order events.
// Athlete events and events of non-users are marked done without action.
// After a create, the worker queues a "recheck" event for the same activity
// two hours later: edits to the description or the race flag send no
// webhook, so this is how points catch up with them.
//
// Failures are retried: rate limits at the next Strava window, other errors
// with backoff up to maxAttempts; a rejected token fails the event at once.
// Shutdown stops the worker between events, never in the middle of one.
package worker
