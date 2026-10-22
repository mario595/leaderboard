// Package webhook serves the Strava webhook endpoint.
//
// Verify answers the GET Strava sends while a subscription is created: it
// echoes hub.challenge only when hub.verify_token matches ours. Receive
// accepts event POSTs: it checks the subscription ID and the fields, stores
// the event in webhook_events, wakes the worker without blocking, and answers
// 200 within Strava's 2-second limit. No Strava calls and no scoring happen
// here; the table is the queue and the worker (#14) processes it.
//
// Strava does not sign events, so the payload is never trusted as data: an
// event only tells the worker to read the activity from Strava with its
// owner's token.
package webhook
