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
- A Strava API client that refreshes expired access tokens and saves the new ones (`internal/strava`); no endpoints used yet
- SQLite storage with versioned migrations
- A health endpoint, structured logging and graceful shutdown
- CI (format, vet, tests, lint)

Not built yet: receiving Strava activity events, the leaderboard and personal pages (a React frontend). The planned work is tracked as issues in this repository and on its [project board](https://github.com/users/mario595/projects/6).

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
```

All settings are environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `STRAVA_CLIENT_ID` | none | Strava application id. Required to run the server. |
| `STRAVA_CLIENT_SECRET` | none | Strava application secret. Required to run the server. Never commit it. |
| `BASE_URL` | `http://localhost:8080` | Public address of the app. Strava redirects back to `BASE_URL/auth/strava/callback`. |
| `ADDR` | `:8080` | Address the server listens on. |
| `ENV` | `dev` | `dev` (text logs) or `prod` (JSON logs, secure cookies). |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `DB_PATH` | `data/leaderboard.db` | SQLite database file. Created, and migrated, on start. |

### 3. Run it

```
make run
```

Then open <http://localhost:8080/auth/strava> and sign in with Strava. You are redirected to `/api/me`, which shows your profile as JSON (there is no frontend yet). Check the server with:

```
curl localhost:8080/healthz
```

### HTTP endpoints

| Method and path | Access | What it does |
|---|---|---|
| `GET /healthz` | public | Health check |
| `GET /auth/strava` | public | Starts the Strava sign-in |
| `GET /auth/strava/callback` | public | Strava redirects here; creates the session |
| `POST /auth/logout` | public | Ends the session (a POST, so another site cannot trigger it) |
| `GET /api/me` | signed in | The current user, without any token |

Everything under `/api` answers `401` with `{"error": "..."}` when nobody is signed in. Sessions are stored in the database and sent in an `HttpOnly`, `SameSite=Lax` cookie (`Secure` when `ENV=prod`) that lasts 30 days.

## Seasons

Seasons are managed from the same binary:

```
go run ./cmd/server season create -name "Autumn 2026" -start 2026-09-01 -end 2026-12-31
go run ./cmd/server season list
```

Both dates are inclusive, the end must be after the start, and seasons cannot overlap. `go run ./cmd/server` or `go run ./cmd/server serve` starts the web server.

## Development

| Command | What it does |
|---|---|
| `make run` | Load `.env` and start the server |
| `make build` | Build `bin/server` |
| `make test` | Run all tests |
| `make vet` | Run `go vet` |
| `make lint` | Run golangci-lint |

CI runs the format check, `go vet`, the tests and the linter on every pull request. Work happens on one branch per ticket (`feature/<number>-<name>`) and lands through a pull request that closes its issue.

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

## Security notes

- The Strava client secret and the users' tokens are secrets. They live in `.env` and in the database file, both git-ignored. They are never logged.
- Tokens are currently stored in plain text in SQLite. Encrypting them at rest is a planned improvement.
