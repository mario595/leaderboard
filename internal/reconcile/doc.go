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
package reconcile
