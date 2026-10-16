// Package scoring implements the leaderboard point rules.
//
// An activity is eligible when its Strava sport type is counted (runs, rides
// and swims, e-bikes excluded; see DisciplineOf) and its moving time is at
// least MinMovingTime. An eligible activity is worth BasePoints, or
// CompetitionPoints when it is a race for its discipline or carries
// CompetitionTag in its description. Score then keeps, per user and per day,
// only the MaxScoringPerDay activities with the most points.
//
// The package is pure: no I/O, no clock, no knowledge of users or seasons.
// Callers map Strava data into Activity, recompute a whole day with Score on
// every change, and filter by season when building the leaderboard.
package scoring
