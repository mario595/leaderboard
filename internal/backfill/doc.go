// Package backfill imports a member's Strava history for a season, so that
// someone who joins (or a season created after people joined) starts with
// their real total instead of zero.
//
// Import pages through the athlete's activity list, which has no
// descriptions, and fetches the detail of an activity only when its
// #competition tag could change its points: a counted sport, long enough,
// and not already a race. Everything else is saved from the summary. Strava
// filters the list by UTC start time while points count by local day, so
// the query is padded by a day on each side and then filtered by local day.
// Every activity goes through activity.Service.Save, which upserts and
// recomputes the day: importing twice changes nothing.
//
// Runner processes jobs from the backfill_jobs table, one (member, season)
// each, with the same pattern as the webhook worker: the table is the queue,
// a channel wakes the runner, and pending jobs survive restarts.
package backfill
