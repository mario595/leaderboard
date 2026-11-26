# Leaderboard

A small web app for my training group: members sign in with their Strava account, the app reads their activities, awards points, and shows a ranking for the season. It is live at <https://ligadaples.fly.dev> and used by the group.

## Why I built it

I wanted to learn Go, and I learn best by building something real people use. My training group already ran a points league, so it was a good fit: small enough to finish, but with the problems a tutorial skips:

- OAuth with a third-party API, plus token refresh.
- Webhooks that can be lost, and the background work needed to recover them.
- Rate limits that are low enough to shape the design.
- A real deployment, and the privacy rules that come with handling other people's data.

## How I built it: AI as a mentor, not a ghostwriter

The project is built with agentic coding, using [Claude Code](https://claude.com/claude-code). The goal was to learn Go, not just to get an app, so the AI's role was set up on purpose. The rules are in [CLAUDE.md](CLAUDE.md).

- **Claude acted as a senior engineer and mentor.** It recommended the stack, split the work into small tickets (GitHub issues on a [project board](https://github.com/users/mario595/projects/6)), and wrote a step-by-step spec for each one, posted on its issue. It also explained where Go differs from the Java and Python I know.
- **I wrote most of the Go backend.** I followed each spec step, then asked for a review, which covered idioms, conventions, pitfalls and why they matter. When I got stuck, Claude explained the problem and pointed the way rather than writing the solution.
- **Test-first pairing (from [#52](https://github.com/mario595/leaderboard/issues/52) on).** Claude writes a failing test for each step, I write the code that makes it pass, and I review the tests too. Tests are checked with mutation checks: break the code on purpose and see a test fail.
- **What Claude wrote:**
  - most of the React frontend;
  - the legal pages;
  - three tickets end to end, Go included, once the core was done: the consent step ([#55](https://github.com/mario595/leaderboard/issues/55)), account deletion ([#56](https://github.com/mario595/leaderboard/issues/56)) and backup retention ([#57](https://github.com/mario595/leaderboard/issues/57));
  - the tests from #52 on;
  - the doc comments, written after review so they describe the final code.

  Commits with Claude's work carry a `Co-Authored-By` line.
- **Process like a team's.** Each ticket has its own branch and a pull request that closes its issue. I review and merge, CI must pass, and `main` deploys automatically.

## Things worth a look

- **Events that can't get lost.** Strava webhooks are stored and acknowledged at once, then processed by a background worker with retries (`internal/webhook`, `internal/worker`). Strava drops events it can't deliver, so a reconcile job compares each member's recent activities with Strava and repairs any differences (`internal/reconcile`).
- **One rate limiter for the whole app.** Every Strava request goes through the same limiter: webhooks, backfills and reconciles. That keeps the app under Strava's 200 requests per 15 minutes (`internal/strava`).
- **Points that are always recomputed.** Saving or deleting an activity recomputes that member's points for the whole day from scratch, never by adding or subtracting. Replays and out-of-order events are therefore harmless (`internal/activity`, `internal/scoring`).
- **One small binary.** The React frontend is embedded in the Go binary. The Docker image is about 6 MB, on a distroless base, and runs on one Fly.io machine for about $2.60 a month.
- **Privacy built in.**
  - Members consent before connecting Strava, and can delete their account from the app.
  - Backups are kept no longer than the policy allows.
  - After a backup is restored, a command deletes again the members who left since it was taken (see [Your data](#your-data) and [Backups](#backups)).

## What it does

What exists today:

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
- Reconciliation: two hours after each new activity the server checks it again, and at startup and every 24 hours it compares each member's last 7 days with Strava, saving what is missing or changed and deleting what is gone (`internal/reconcile`)
- A leaderboard API: the ranking of all members by points for the current or a chosen season (`GET /api/leaderboard`)
- A web frontend (React, TypeScript, Vite) with the Strava login page, the leaderboard, the member's own activities (grouped by day, with the reason when an activity scored nothing) and the scoring rules (`web/`), embedded in the Go binary: one binary serves the pages and the API (`internal/spa`)
- A personal activity list: each of the member's activities in a season, newest first, with points, the competition flag and why an activity scored nothing, paginated (`GET /api/me/activities`)
- SQLite storage with versioned migrations
- Account deletion from the app, with the Strava access revoked and a written confirmation page
- Backups that honour deletions: snapshots kept 14 days, manual copies 30 days at most, and `server prune-revoked` to delete again, after a restore, the members who left since the backup (see [Backups](#backups))
- A privacy policy and terms of use, public and linked from every page with the support contact (see [Legal pages](#legal-pages)), and a consent step before a new member connects Strava (`/join`)
- A health endpoint, structured logging and graceful shutdown; a request the browser abandons is logged at Debug with status 499, not as a server error
- CI (format, vet, tests, lint)
- A Docker image of about 6 MB to download: the frontend and a static binary on a distroless base, with the database on a volume

The planned work is tracked as issues in this repository and on its [project board](https://github.com/users/mario595/projects/6).

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
- [Docker](https://docs.docker.com/engine/install/), only to build and run the image

No C compiler is needed: the SQLite driver is pure Go. (`go test -race` does need one.)

## Getting started

### 1. Create a Strava API application

1. Go to <https://www.strava.com/settings/api> and create an application.
2. Set the **Authorization Callback Domain** to `localhost` (production uses its own domain; Strava always allows `localhost`, so one application serves both, see [Deploy](#deploy)).
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

### 6. Run it in Docker

The image is what a deployment runs: three build stages (Node builds the frontend, Go builds a static binary that embeds it, `CGO_ENABLED=0`), and a final image with only the binary on [distroless](https://github.com/GoogleContainerTools/distroless) `static:nonroot`, which adds CA certificates for the Strava API and runs as a non-root user (UID 65532). It is about 6 MB compressed.

```
make docker-build
make docker-run
```

`make docker-run` passes `.env` with `--env-file`, which reads `KEY=value` lines literally: quotes are kept and `export` is not understood. It sets `BASE_URL=http://localhost:8080`, since `.env` points to the dev server. The image defaults to `ENV=prod` (JSON logs, secure cookies, which browsers accept on `http://localhost`). The database is `/data/leaderboard.db` in the `leaderboard-data` volume: it outlives the container, which is removed when it stops. A new volume starts empty; mount a named volume, not `./data`, which belongs to your user and is not writable by UID 65532.

The CLI runs through the same image, against the same volume, also while the server runs:

```
docker run --rm --env-file .env -e BASE_URL=http://localhost:8080 -v leaderboard-data:/data leaderboard season list
```

The image has no shell, so `docker exec` cannot open one: use the CLI as above, and `docker logs -f leaderboard` for the logs. `docker stop` sends SIGTERM and the server shuts down gracefully. `.dockerignore` keeps `.env`, the local database and `.git` out of the build.

### HTTP endpoints

| Method and path | Access | What it does |
|---|---|---|
| `GET /healthz` | public | Health check |
| `GET /auth/strava` | public | Starts the Strava sign-in |
| `GET /auth/strava/callback` | public | Strava redirects here; logs a member in, or signs up a new one with the invitation from the session, and creates the session |
| `GET /invite/{code}` | public | An invitation link: keeps the code in the session and sends the browser to the consent page (`/join`), whose button starts the Strava sign-in |
| `POST /auth/logout` | public | Ends the session (a POST, so another site cannot trigger it) |
| `GET /api/me` | signed in | The current user, without any token |
| `DELETE /api/me` | signed in | Deletes the current user's account: revokes the app on Strava, deletes everything about them and every session, and answers with the confirmation (`deletedAt`, `backupsGoneBy`) |
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

Production holds the application's only subscription (see [Deploy](#deploy)); locally, POST a sample event to `http://localhost:8080/webhook/strava` instead. To receive real events locally, the subscription must point to your machine, which must be reachable from the internet. Any tunnel works:

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

`make strava-subscription` shows the current subscription and `make strava-unsubscribe ID=<id>` deletes it. While the server is down, Strava's deliveries fail and, after a few retries, are lost; the next reconcile (at startup) recovers them.

Strava sends no event when a member edits the description (`#competition`) or changes the workout type to Race. Two hours after each new activity the worker fetches it again, which catches the usual edit made right after the upload; later edits are caught by the daily reconcile of the last 7 days. The reconcile lists each member's activities (one request), fetches the details that can change points, as the backfill does, and logs `reconcile pass`, then `reconcile done` per member with `synced` (activities saved, changed or not) and `deleted`.

## Deploy

Production runs on [Fly.io](https://fly.io) at <https://ligadaples.fly.dev>: one always-on machine (`shared-cpu-1x`, 256 MB) in Paris (`cdg`) with a 1 GB volume for the database, about $2.60 a month. Fly builds the `Dockerfile`, serves HTTPS, and checks `/healthz`. The settings are in `fly.toml`.

- **Always on, one machine.** `auto_stop_machines = "off"`: a stopped machine would run no worker, backfill or reconcile. Never scale beyond one machine: the volume attaches to one, and SQLite wants a single writer.
- **Deploys.** Every push to `main` deploys from CI, after the tests, linter and frontend checks pass (`deploy` job, with a deploy-only token in the `FLY_API_TOKEN` repository secret). A manual `fly deploy` builds from the local folder, uncommitted changes included. A deploy replaces the machine: a few seconds offline, which Strava's webhook retries and the reconcile cover.
- **Configuration.** Non-secret settings in `fly.toml` (`BASE_URL`, `LOG_LEVEL`; the image sets `ENV=prod`, `DB_PATH`, `ADDR`). Secrets with `fly secrets`: `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET`, `STRAVA_VERIFY_TOKEN` (its own, not the dev one) and `STRAVA_SUBSCRIPTION_ID`. Load them from stdin so they stay out of the shell history: `grep -E '^STRAVA_CLIENT_(ID|SECRET)=' .env | fly secrets import`.
- **Strava.** One Strava application serves both: its Authorization Callback Domain is `ligadaples.fly.dev`, and Strava always allows `localhost`, so local login keeps working. The single webhook subscription points to production; to recreate it with the production token, start the line with a space to keep it out of the history:
  ```
   VERIFY_TOKEN=<production token> make strava-subscribe CALLBACK=https://ligadaples.fly.dev/webhook/strava
  ```
  Before inviting members, raise the application from single-player mode to 10 athletes on Strava's API settings page.
- **CLI.** The image has no shell; run a command directly: `fly ssh console -C "/server season list"`, and the same for `season create`, `invite create`, `backfill`. It sees the production settings, so invitation links start with the production URL.
- **Logs.** `fly logs` follows them live (JSON lines); the dashboard keeps recent logs only.
- **Shutdown.** Fly sends SIGTERM and allows 30 seconds (`kill_signal`, `kill_timeout`), enough for the graceful shutdown.

### Backups

Backups hold the members' Strava data and tokens. A member who leaves must be gone from every copy within 30 days (Strava API Policy 7.4(b)), and the privacy policy promises it.

- **Automatic:** Fly snapshots the volume every day and keeps 14 days (`fly volumes snapshots list <volume id>`; `snapshot_retention` in `fly.toml`).
- **Manual, before risky changes** (a migration, a data fix, the end of a season): `server backup` writes a consistent copy with SQLite's `VACUUM INTO`, while the app runs. Write it to `/tmp`, which is not on the volume, download it, then restart the machine: `/tmp` is only cleared on a restart, the image has no `rm`, and Fly's sftp cannot delete files. The restart takes a few seconds; webhooks missed meanwhile come back with the reconcile job.
  ```
  fly ssh console -C "/server backup -out /tmp/backup-YYYY-MM-DD.db"
  fly ssh sftp get /tmp/backup-YYYY-MM-DD.db ~/leaderboard-backups/backup-YYYY-MM-DD.db
  fly machine restart
  ```
  The command refuses an existing file. Keep the copies only in `~/leaderboard-backups/` on the operator's machine: never in the repo (`*.db` is git-ignored), an issue or a chat. **Delete each copy within 30 days** (deleting the file is enough):
  ```
  find ~/leaderboard-backups -name 'backup-*.db' -mtime +30 -delete
  ```
- **What a total loss costs:** activities and points come back with a backfill from Strava, seasons are recreated with the CLI; members need a new invitation link and one login each.

Restore from a snapshot:

1. Create a volume from it (`fly volumes create leaderboard_data --snapshot-id <snapshot id> --region cdg --size 1`), then run the machine on the new volume instead of the old one (see Fly's [volume snapshot docs](https://fly.io/docs/volumes/snapshots/)).
2. **Delete again the members who left after the snapshot was taken.** The restored database has their data back and nothing in it remembers they left, but Strava does: their authorization is gone, whichever way they left. List them first, then delete:
   ```
   fly ssh console -C "/server prune-revoked"
   fly ssh console -C "/server prune-revoked -delete"
   ```
   It makes one Strava call (`GET /athlete`) per member. Only a rejected authorization (`ErrUnauthorized`) counts as revoked; a rate limit or a network error is printed and the member is kept, so run it again. If Strava rejects every member it refuses to delete: that means the app's `STRAVA_CLIENT_ID`/`STRAVA_CLIENT_SECRET` are wrong, not that the whole group left.

These steps have not been rehearsed yet: try them once on a scratch app before relying on them. A downloaded backup is mainly an off-site copy; restoring it would mean uploading it with `fly ssh sftp put` and swapping it in while the app is stopped, which the image (no shell) does not support yet.

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
| `make docker-build` | Build the Docker image `leaderboard` |
| `make docker-run` | Run the image on port 8080 with `.env` and the `leaderboard-data` volume |

CI runs the Go checks (format, `go vet`, tests, linter) and `make web-check`'s frontend checks on every pull request, and deploys `main` once they pass. Work happens on one branch per ticket (`feature/<number>-<name>`) and lands through a pull request that closes its issue.

### Project layout

```
cmd/server/            entry point: web server and the `season`, `invite` and `backfill` subcommands
internal/activity/     saving and deleting activities, with the per-day point recompute
internal/auth/         Strava OAuth login and callback, invitation links, logout
internal/backfill/     background import of a member's season from Strava
internal/config/       settings read from the environment
internal/httpapi/      router, middleware, JSON API handlers
internal/logging/      structured logger setup
internal/reconcile/    startup and daily check of recent activities against Strava
internal/scoring/      the point rules (pure functions)
internal/spa/          serves the embedded frontend build
internal/store/        SQLite access, embedded SQL migrations, one store per table
internal/strava/       Strava API client with token refresh
internal/testutil/     shared test assertions
internal/webhook/      Strava webhook endpoint
internal/worker/       background processing of webhook events
web/                   React frontend (Vite, TypeScript); builds into internal/spa/dist/
Dockerfile             multi-stage image build (frontend, Go binary, distroless runtime)
fly.toml               Fly.io settings: machine, volume, health check
```

### Stack

Go with the standard library's `net/http` and the [chi](https://github.com/go-chi/chi) router, SQLite through `database/sql` (pure-Go driver [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) with [goose](https://github.com/pressly/goose) migrations, and [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) for Strava. The frontend is React with TypeScript, built by [Vite](https://vite.dev) and embedded in the binary with `go:embed`.

## Your data

For each member the app stores:

- the Strava profile basics (athlete ID, first and last name), the OAuth tokens that let it read activities, and which invitation they signed up with;
- for each counted activity: sport type, local start time, moving time, distance, workout type, description (only to find the `#competition` tag) and the points awarded;
- the webhook events Strava sends about them, until processed;
- the backfill jobs that import their seasons.

A member can delete all of it in two ways:

- **In the app:** "Desconectar y borrar mis datos" at the end of "Mis actividades" (`DELETE /api/me`). The server revokes the app's access on Strava (`POST /oauth/deauthorize`), then deletes the member, their activities, events and backfill jobs, and every session they have on any device. A failed revoke is logged and the deletion goes on. The member lands on `/account-deleted`, which is the written confirmation Strava requires (API Policy 2.5): the app has no email address to send one to. It shows what was deleted, when, and the date by which backups no longer hold it (30 days, the longest any backup is kept: see [Backups](#backups)). Strava then sends a deauthorization event, which finds nobody left.
- **On Strava** (Settings → My Apps → Revoke Access): Strava sends a deauthorization event, and the app removes the member, their activities and their events in one transaction. Any open session of that member stops working on its next request. The member isn't in the app at that moment, so the privacy policy is the confirmation, plus an email on request.

Signing in again starts from scratch, and needs a new invitation.

## Legal pages

The privacy policy (`/privacy`) and the terms of use (`/terms`) are public pages of the frontend, in Spanish: `web/src/pages/PrivacyPage.tsx` and `web/src/pages/TermsPage.tsx`. The support contact, the data controller and the "last updated" date live in `web/src/legal.ts`. Every page links to them from its footer, together with the support email and, for a logged-in member, their Strava profile (Strava API Policy 2.4).

An invitation link lands on the consent page, `/join` (`web/src/pages/JoinPage.tsx`), before Strava. It summarises what is read and how, who sees it, how to withdraw and how to ask for deletion (Strava API Policy 2.1), and the "Connect with Strava" button only becomes a link once the athlete agrees that the group sees their name and points (5.13). Consent is not stored, and members who signed up before it existed are not asked.

Keep them in sync with [Your data](#your-data). Any change to what the app collects needs:
- both pages and the consent page updated, and `policyUpdated` changed;
- members told, and their consent asked for again before the new data is read (Strava API Policy 7.2).

`web/src/pages/LegalPages.test.tsx` pins the clauses the Strava API Policy and the GDPR require: the usage data statement (6.5), deferring to Strava's privacy policy (7.3), the third-party disclaimers (9.2), the legal basis and the retention.

## Security notes

- The Strava client secret and the users' tokens are secrets. They live in `.env` and in the database file, both git-ignored, and in production in `fly secrets` and the volume. They are never logged. Database backups contain the tokens.
- Tokens are currently stored in plain text in SQLite. Encrypting them at rest is a planned improvement.
- An invitation code works like a shared password with a limited number of uses: share links only with the group, and revoke one that leaks (`invite revoke`). Codes are never logged.
