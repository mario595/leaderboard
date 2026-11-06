// Package reconcile repairs what the webhooks missed for a member's recent
// activities: creates, updates and deletes sent while the server was down,
// and edits Strava sends no webhook for (a description with #competition, a
// workout type changed to Race).
//
// User imports a window of days with backfill.Import, which saves every
// activity Strava lists and fetches the details that can change points, then
// deletes the stored activities of that window that the import did not save.
// After a failed import it deletes nothing: a partial list would look like
// deletions.
//
// Scheduler runs User for every member over the last week: once at startup,
// to recover what was missed while the server was down, then every 24 hours.
//
// A webhook create processed between the import's list and the read of the
// stored IDs makes that new activity look deleted. The window is seconds, and
// the recheck the worker schedules two hours after each create saves it again.
package reconcile
