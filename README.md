# Leaderboard

A small web app for a training group: members sign in with their Strava account, the app reads their activities, awards points, and shows a ranking.

This is also a **learning project** for Go. The code is written step by step, ticket by ticket, with a focus on idiomatic Go and good practices. See [CLAUDE.md](CLAUDE.md) for how the work is organized.

## Status

Work in progress. What exists today:

- Sign in with Strava (OAuth), which updates a member's name and tokens; new members need an invitation link to sign up, managed with a CLI
- Cookie sessions kept in the database, logout, and a first protected endpoint (`GET /api/me`)
- A season model (a named range of dates) with a CLI to create and list seasons
- The scoring rules below, as a tested Go package (`internal/scoring`)
- Activity storage with points recomputed per user and day on every save or delete (`internal/activity`)
- A Strava API client that refreshes expired access tokens and saves the new ones, fetches an activity or a page of a member's activities, with errors for not found, revoked access and rate limits, and paces every request through one app-wide rate limiter (`internal/strava`)
- A Strava webhook endpoint that verifies the subscription and stores incoming events (`internal/webhook`)
- A background worker that turns those events into scored activities: it fetches each activity from Strava and saves or deletes it, with retries (`internal/worker`)
- Deauthorization: when a member revokes the app on Strava, everything stored about them is deleted
- Season backfill: a new member's activities since the season started are imported in the background, and `server backfill` imports a season for every member (`internal/backfill`)
- A leaderboard API: the ranking of all members by points for the current or a chosen season (`GET /api/leaderboard`)
- A web frontend (React, TypeScript, Vite) with the Strava login page, the leaderboard, the member's own activities (grouped by day, with the reason when an activity scored nothing) and the scoring rules (`web/`), embedded in the Go binary: one binary serves the pages and the API (`internal/spa`)
- A personal activity list: each of the member's activities in a season, newest first, with points, the competition flag and why an activity scored nothing, paginated (`GET /api/me/activities`)
- SQLite storage with versioned migrations
- A health endpoint, structured logging and graceful shutdown
- CI (format, vet, tests, lint)

Not built yet: recovering webhook events missed while the server was down, and a deployment. The planned work is tracked as issues in this repository and on its [project board](https://github.com/users/mario595/projects/6).

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

### 5. Run the built binary

In production one binary serves the pages and the API on the same port: `make build` builds the frontend into `internal/spa/dist/` and then the Go binary, which embeds it (`go:embed`). To try it locally, with Strava sending you back to port 8080 instead of the dev server:

```
make build
(set -a; . ./.env; set +a; BASE_URL=http://localhost:8080 ./bin/server)
```

Open <http://localhost:8080>. Any path that is not a file, the API, the webhook or the health check gets the app's `index.html`, so reloading `/activities` works. A missing file (`/assets/old.js`) is a 404. Files under `/assets/` have a content hash in their names and are cached for a year; `index.html` is always revalidated, so a new deploy shows up on the next load. A binary built without the frontend (plain `go build`) answers pages with `503` and a hint to run `make web-build`.

### HTTP endpoints

| Method and path | Access | What it does |
|---|---|---|
| `GET /healthz` | public | Health check |
| `GET /auth/strava` | public | Starts the Strava sign-in |
| `GET /auth/strava/callback` | public | Strava redirects here; logs a member in, or signs up a new one with the invitation from the session, and creates the session |
| `GET /invite/{code}` | public | An invitation link: keeps the code in the session and starts the Strava sign-in |
| `POST /auth/logout` | public | Ends the session (a POST, so another site cannot trigger it) |
| `GET /api/me` | signed in | The current user, without any token |
| `GET /api/me/activities` | signed in | The member's own activities in the season (default: current; `?season=<id>`), newest first, every activity with its points (often 0), competition flag and `zeroReason` (why it scored nothing: `sport`, `short` or `daily_cap`; `""` when it scored). Pages of `?limit=` (1 to 100, default 50); pass the response's `nextCursor` as `?cursor=` for the next page (`null` on the last one). Moving time in seconds, distance in metres. |
| `GET /api/leaderboard` | signed in | Ranking for the season covering today (UTC date); `?season=<id>` for another season. Every member is listed, with 0 points if they have none; ties share a rank (1, 2, 2, 4). `404` when no season covers today. |
| `GET /webhook/strava` | public | Strava's subscription check: echoes the challenge if the verify token matches |
| `POST /webhook/strava` | public | Strava webhook events; stored, then processed in the background |
| `GET /*` | public | The frontend: a built file, or `index.html` for any other path (the pages check the session themselves through `/api/me`) |

Everything under `/api` answers `401` with `{"error": "..."}` when nobody is signed in. Sessions are stored in the database and sent in an `HttpOnly`, `SameSite=Lax` cookie (`Secure` when `ENV=prod`) that lasts 30 days.

## Seasons

Seasons are managed from the same binary:

```
go run ./cmd/server season create -name "Autumn 2026" -start 2026-09-01 -end 2026-12-31
go run ./cmd/server season list
```

Both dates are inclusive, the end must be after the start, and seasons cannot overlap. `go run ./cmd/server` or `go run ./cmd/server serve` starts the web server.

## Invitations

Only people with an invitation link can create an account; members who already have one sign in with the Strava button. Invitations are managed from the same binary:

```
go run ./cmd/server invite create -uses 10 -expires 168h
go run ./cmd/server invite list
go run ./cmd/server invite revoke -code <code>
```

`create` prints the link to share, `BASE_URL/invite/<code>`: load `.env` first (`set -a; . ./.env; set +a`) so the link points at the right host. `-uses` is how many people can sign up with it (required); `-expires` is how long it stays valid (`168h` is a week; default: never). `list` shows each code with its uses, expiry and whether it was revoked. A new athlete who signs in without a usable invitation (none, unknown, used up, revoked or expired) gets no account and is sent back to the login page with an explanation. Each member records the invitation they signed up with.

## Backfill

A member who signs up during a season starts with their real total: the server imports their Strava activities since the season started, in the background, while they already use the app. Their page fills in as the import goes.

To import a season for every member (a season created after people joined, or to redo a failed import), with the server running:

```
go run ./cmd/server backfill -season <id>
```

It only queues one job per member; the running server picks them up within a minute. Running it twice is harmless: a job that is still waiting is not queued again, and importing the same activities again changes nothing.

How it works: the app lists the member's activities for the season's dates (one request per 200 activities) and fetches the full activity only when its `#competition` tag could add points (a counted sport, 30 minutes or more, and not already a race), since the list has no descriptions. Every Strava request of the app, webhooks included, goes through one rate limiter: one request every 6 seconds, in bursts of up to 5, which keeps the app under Strava's limit of 200 requests per 15 minutes. A season with an activity a day takes a few minutes per month. If Strava still answers "rate limited", the job waits for the next 15-minute window. Stopping the server stops an import at its next request; it starts over on the next start. Progress is logged: `backfill started` and `backfill done` (with counts and duration) at the default level, each page and fetch with `LOG_LEVEL=debug`.

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
| `make build` | Build the frontend, then `bin/server` with the frontend embedded |
| `make test` | Run all tests |
| `make vet` | Run `go vet` |
| `make lint` | Run golangci-lint |
| `make web-install` | Install the frontend dependencies (`npm ci`) |
| `make web-dev` | Start the Vite dev server on port 5173 |
| `make web-test` | Run the frontend tests (Vitest) |
| `make web-check` | Everything CI checks on the frontend: lint, type-check, format, tests, build |
| `make web-build` | Build the frontend into `internal/spa/dist/` (embedded by `make build`) |

CI runs the Go checks (format, `go vet`, tests, linter) and `make web-check`'s frontend checks on every pull request. Work happens on one branch per ticket (`feature/<number>-<name>`) and lands through a pull request that closes its issue.

### Project layout

```
cmd/server/            entry point: web server and the `season`, `invite` and `backfill` subcommands
internal/activity/     saving and deleting activities, with the per-day point recompute
internal/auth/         Strava OAuth login and callback, invitation links, logout
internal/backfill/     background import of a member's season from Strava
internal/config/       settings read from the environment
internal/httpapi/      router, middleware, JSON API handlers
internal/logging/      structured logger setup
internal/scoring/      the point rules (pure functions)
internal/spa/          serves the embedded frontend build
internal/store/        SQLite access, embedded SQL migrations, one store per table
internal/strava/       Strava API client with token refresh
internal/testutil/     shared test assertions
internal/webhook/      Strava webhook endpoint
internal/worker/       background processing of webhook events
web/                   React frontend (Vite, TypeScript); builds into internal/spa/dist/
```

### Stack

Go with the standard library's `net/http` and the [chi](https://github.com/go-chi/chi) router, SQLite through `database/sql` (pure-Go driver [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) with [goose](https://github.com/pressly/goose) migrations, and [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) for Strava. The frontend is React with TypeScript, built by [Vite](https://vite.dev) and embedded in the binary with `go:embed`.

## Your data

For each member the app stores:

- the Strava profile basics (athlete ID, first and last name), the OAuth tokens that let it read activities, and which invitation they signed up with;
- for each counted activity: sport type, local start time, moving time, distance, workout type, description (only to find the `#competition` tag) and the points awarded;
- the webhook events Strava sends about them, until processed;
- the backfill jobs that import their seasons.

Revoking the app in Strava (Settings → My Apps → Revoke Access) deletes all of it: Strava sends a deauthorization event, and the app removes the member, their activities and their events in one transaction. Any open session of that member stops working on its next request. Signing in again starts from scratch.

## Security notes

- The Strava client secret and the users' tokens are secrets. They live in `.env` and in the database file, both git-ignored. They are never logged.
- Tokens are currently stored in plain text in SQLite. Encrypting them at rest is a planned improvement.
- An invitation code works like a shared password with a limited number of uses: share links only with the group, and revoke one that leaks (`invite revoke`). Codes are never logged.
