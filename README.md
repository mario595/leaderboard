# Leaderboard

A small web app for a training group: members sign in with their Strava account, the app reads their activities, awards points, and shows a ranking.

This is also a **learning project** for Go. The code is written step by step, ticket by ticket, with a focus on idiomatic Go and good practices. See [CLAUDE.md](CLAUDE.md) for how the work is organized.

## Status

Work in progress. What exists today:

- Sign in with Strava (OAuth), which creates or updates the user and stores the tokens
- Cookie sessions kept in the database, logout, and a first protected endpoint (`GET /api/me`)
- A season model (a named range of dates) with a CLI to create and list seasons
- The scoring rules below, as a tested Go package (`internal/scoring`)
- Activity storage with points recomputed per user and day on every save or delete (`internal/activity`); not fed by Strava yet
- A Strava API client that refreshes expired access tokens and saves the new ones, and fetches an activity with errors for not found, revoked access and rate limits (`internal/strava`); not called by the app yet
- A Strava webhook endpoint that verifies the subscription and stores incoming events (`internal/webhook`)
- A background worker that turns those events into scored activities: it fetches each activity from Strava and saves or deletes it, with retries (`internal/worker`)
- Deauthorization: when a member revokes the app on Strava, everything stored about them is deleted
- A leaderboard API: the ranking of all members by points for the current or a chosen season (`GET /api/leaderboard`)
- A web frontend (React, TypeScript, Vite) with the Strava login page, the leaderboard, the member's own activities (grouped by day, with the reason when an activity scored nothing) and the scoring rules (`web/`)
- A personal activity list: each of the member's activities in a season, newest first, with points, the competition flag and why an activity scored nothing, paginated (`GET /api/me/activities`)
- SQLite storage with versioned migrations
- A health endpoint, structured logging and graceful shutdown
- CI (format, vet, tests, lint)

Not built yet: serving the frontend from the Go binary, and loading activities from before signing up. The planned work is tracked as issues in this repository and on its [project board](https://github.com/users/mario595/projects/6).

## How points work

The rules, implemented in `internal/scoring`:

- Counted activity types: runs (including trail and virtual), rides (including mountain bike, gravel and virtual), and swims. E-bikes are excluded.
- An activity must have at least **30 minutes of moving time**.
- Each activity is worth **2 points**, or **4 points** if it is a competition (a Strava "Race" workout type, or `#competition` in the activity description).
- At most **2 activities per user per day** score, the ones with the most points; on a tie, the one that started first. The day is the activity's local start date.
- Only activities inside the current **season** count.

## Requirements

- [Go](https://go.dev/dl/) (the version in `go.mod`)
- [golangci-lint](https://golangci-lint.run/welcome/install/) v2, only for `make lint`
- [Node.js](https://nodejs.org/) (the major version in `web/.nvmrc`), only for the frontend
- A Strava account and a Strava API application (see below)

No C compiler is needed: the SQLite driver is pure Go. (`go test -race` does need one.)

## Getting started

### 1. Create a Strava API application

1. Go to <https://www.strava.com/settings/api> and create an application.
2. Set the **Authorization Callback Domain** to `localhost`.
3. Note the **Client ID** and **Client Secret**. Keep the secret private.

A new Strava application starts in single-player mode: only the account that owns the application can sign in. Strava lets you raise that limit from the same page.

### 2. Configure the environment

Copy `.env.example` to `.env` (it is git-ignored) and fill in the values:

```
STRAVA_CLIENT_ID=...
STRAVA_CLIENT_SECRET=...
BASE_URL=http://localhost:8080
STRAVA_VERIFY_TOKEN=...      # openssl rand -hex 16
STRAVA_SUBSCRIPTION_ID=      # set after creating the webhook subscription
```

All settings are environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `STRAVA_CLIENT_ID` | none | Strava application id. Required to run the server. |
| `STRAVA_CLIENT_SECRET` | none | Strava application secret. Required to run the server. Never commit it. |
| `STRAVA_VERIFY_TOKEN` | none | Secret checked when Strava validates the webhook callback. Required to run the server. |
| `STRAVA_SUBSCRIPTION_ID` | unset | ID of our Strava webhook subscription. Events for any other ID are rejected; while unset, all events are. |
| `BASE_URL` | `http://localhost:8080` | Public address of the app. Strava redirects back to `BASE_URL/auth/strava/callback`. |
| `ADDR` | `:8080` | Address the server listens on. |
| `ENV` | `dev` | `dev` (text logs) or `prod` (JSON logs, secure cookies). |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `DB_PATH` | `data/leaderboard.db` | SQLite database file. Created, and migrated, on start. |

### 3. Run it

```
make run
```

Check the server with:

```
curl localhost:8080/healthz
```

### 4. Run the frontend

The frontend lives in `web/`. In development the Vite dev server serves it on port 5173 and forwards `/api` and `/auth` to the Go server, so the browser only talks to one origin (no CORS) and the session cookie just works.

1. Set `BASE_URL=http://localhost:5173` in `.env`, so Strava sends you back through the dev server after logging in. (`.env` wins over a value given on the command line.)
2. Install the dependencies once: `make web-install`.
3. Run the Go server (`make run`) and, in a second terminal, `make web-dev`.
4. Open <http://localhost:5173> and sign in with Strava.

Until the frontend is served by the Go binary (#21), <http://localhost:8080> has no pages, only the API.

### HTTP endpoints

| Method and path | Access | What it does |
|---|---|---|
| `GET /healthz` | public | Health check |
| `GET /auth/strava` | public | Starts the Strava sign-in |
| `GET /auth/strava/callback` | public | Strava redirects here; creates the session |
| `POST /auth/logout` | public | Ends the session (a POST, so another site cannot trigger it) |
| `GET /api/me` | signed in | The current user, without any token |
| `GET /api/me/activities` | signed in | The member's own activities in the season (default: current; `?season=<id>`), newest first, every activity with its points (often 0), competition flag and `zeroReason` (why it scored nothing: `sport`, `short` or `daily_cap`; `""` when it scored). Pages of `?limit=` (1 to 100, default 50); pass the response's `nextCursor` as `?cursor=` for the next page (`null` on the last one). Moving time in seconds, distance in metres. |
| `GET /api/leaderboard` | signed in | Ranking for the season covering today (UTC date); `?season=<id>` for another season. Every member is listed, with 0 points if they have none; ties share a rank (1, 2, 2, 4). `404` when no season covers today. |
| `GET /webhook/strava` | public | Strava's subscription check: echoes the challenge if the verify token matches |
| `POST /webhook/strava` | public | Strava webhook events; stored, then processed in the background |

Everything under `/api` answers `401` with `{"error": "..."}` when nobody is signed in. Sessions are stored in the database and sent in an `HttpOnly`, `SameSite=Lax` cookie (`Secure` when `ENV=prod`) that lasts 30 days.

## Seasons

Seasons are managed from the same binary:

```
go run ./cmd/server season create -name "Autumn 2026" -start 2026-09-01 -end 2026-12-31
go run ./cmd/server season list
```

Both dates are inclusive, the end must be after the start, and seasons cannot overlap. `go run ./cmd/server` or `go run ./cmd/server serve` starts the web server.

## Strava webhooks

Strava notifies the app when a member creates, edits or deletes an activity, by calling a public URL: one subscription per Strava application. The server only stores each event and answers; processing happens in the background.

For local development the server must be reachable from the internet. Any tunnel works:

- **A named Cloudflare tunnel on your own domain** (recommended): the URL is stable, so the subscription is created once. Run `cloudflared` as a service with an `ingress` rule for your hostname pointing to `http://localhost:8080` and a final `http_status:404` rule.
- **A quick tunnel**, no account needed: `cloudflared tunnel --url http://localhost:8080` prints a random `https://<name>.trycloudflare.com` URL. It changes on every run, so delete and recreate the subscription each time.

If the domain uses Cloudflare Access or Bot Fight Mode, make sure they do not cover this hostname: Strava cannot pass a login page or a challenge.

Create the subscription:

1. Start the server (`make run`) and the tunnel. Check `https://<host>/healthz`.
2. Subscribe. Strava calls the callback during this request to check `STRAVA_VERIFY_TOKEN`:
   ```
   make strava-subscribe CALLBACK=https://<host>/webhook/strava
   ```
   The response contains the subscription `id`.
3. Put the `id` in `.env` as `STRAVA_SUBSCRIPTION_ID` and restart the server.
4. Edit an activity on Strava: change its title (Strava only sends updates for the title, sport type and privacy). The server stores the event, the worker fetches the activity and saves it with its points, and the event becomes `done`.

`make strava-subscription` shows the current subscription and `make strava-unsubscribe ID=<id>` deletes it. While the server is down, Strava's deliveries fail and, after a few retries, are lost.

## Development

| Command | What it does |
|---|---|
| `make run` | Load `.env` and start the server |
| `make build` | Build `bin/server` |
| `make test` | Run all tests |
| `make vet` | Run `go vet` |
| `make lint` | Run golangci-lint |
| `make web-install` | Install the frontend dependencies (`npm ci`) |
| `make web-dev` | Start the Vite dev server on port 5173 |
| `make web-test` | Run the frontend tests (Vitest) |
| `make web-check` | Everything CI checks on the frontend: lint, type-check, format, tests, build |
| `make web-build` | Build the frontend into `web/dist/` |

CI runs the Go checks (format, `go vet`, tests, linter) and `make web-check`'s frontend checks on every pull request. Work happens on one branch per ticket (`feature/<number>-<name>`) and lands through a pull request that closes its issue.

### Project layout

```
cmd/server/            entry point: web server and the `season` subcommands
internal/auth/         Strava OAuth login and callback
internal/config/       settings read from the environment
internal/httpapi/      router, health endpoint, middleware
internal/logging/      structured logger setup
internal/store/        SQLite access, embedded SQL migrations, users and seasons
```

### Stack

Go with the standard library's `net/http` and the [chi](https://github.com/go-chi/chi) router, SQLite through `database/sql` (pure-Go driver [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) with [goose](https://github.com/pressly/goose) migrations, and [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) for Strava. A React frontend is planned.

## Your data

For each member the app stores:

- the Strava profile basics (athlete ID, first and last name) and the OAuth tokens that let it read activities;
- for each counted activity: sport type, local start time, moving time, distance, workout type, description (only to find the `#competition` tag) and the points awarded;
- the webhook events Strava sends about them, until processed.

Revoking the app in Strava (Settings → My Apps → Revoke Access) deletes all of it: Strava sends a deauthorization event, and the app removes the member, their activities and their events in one transaction. Any open session of that member stops working on its next request. Signing in again starts from scratch.

## Security notes

- The Strava client secret and the users' tokens are secrets. They live in `.env` and in the database file, both git-ignored. They are never logged.
- Tokens are currently stored in plain text in SQLite. Encrypting them at rest is a planned improvement.
