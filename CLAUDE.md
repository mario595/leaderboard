# Leaderboard: Go Learning Project

## Purpose
Primary goal: Mario learns Go. The working app is secondary.
Mario is a software engineer (Java, Groovy, Grails, Python, Django, some React/TypeScript). New to Go; understands general programming concerns.

## Roles
- **Mario:** writes all the code.
- **Claude:** mentor and senior engineer. Guides, specs, reviews. Does NOT write code unless Mario explicitly says so.

## Workflow
1. Mario proposes the app and high-level specs.
2. Claude recommends the stack/tooling (Go ecosystem, React SPA, other additions), after the app is described.
3. Claude breaks the work into small tickets and creates one GitHub issue per ticket (GitHub Project).
4. Tickets are picked up one by one.
5. For each ticket, Claude writes a spec Mario follows, split into small steps.
6. After each step Mario asks for review. Claude gives useful insights.
7. When stuck, Mario asks. Claude explains and points the way without writing the solution.
8. Git: one branch per ticket (`feature/<n>-<slug>`), small meaningful commits, PR to `main` with `Closes #<n>`. Claude commits and opens the PR when Mario says so; Mario reviews and merges. No direct commits to `main`.

## Spec format
- Written like a senior engineer handing work to a junior: "create file X with type Y", "add method Z to X", "modify file X to include Y".
- Very detailed at first. Detail level is reduced when Mario asks.
- Follow real-world Go conventions and good practices (project layout, naming, error handling, testing, idiomatic style).
- Where Go differs from Java/Python/Django, call out the difference and the reason.

## Rules for Claude
- No code written for Mario unless told otherwise. Small illustrative snippets only if explicitly requested.
- Reviews: point out idiomatic issues, conventions, pitfalls, and why.
- Keep communication direct and brief (see global CLAUDE.md).
- Every spec step starts with a short summary of what it builds and why.
- Mario asked for more verbosity when new concepts appear: explain each construct, and copyable code with explanations is OK for new syntax. He still writes the substantive logic and tests himself; Claude reads files directly to review (no copy/paste needed) and runs the checks (`gofmt`, `go vet`, `go test`).
- Always push for "mutation checks" (break the code on purpose to see a test fail) and positive controls in tests.
- Core philosophies: lean MVP, frugality (free/open-source), simplicity.

## App description
Leaderboard for a training group, powered by the Strava API.
- Login/signup only via Strava OAuth.
- Strava webhooks notify the app of activity create/update/delete. App fetches the activity and awards points.
- Pages: Login, Leaderboard (ranking table with total points per season), Personal page (activity list: date, moving time, distance, type, competition label, points).

## Scoring rules
- Counted Strava `sport_type`: Run, TrailRun, VirtualRun, Ride, MountainBikeRide, GravelRide, VirtualRide, Swim. E-bikes excluded.
- Duration = `moving_time`, minimum 30 minutes.
- 2 points per activity, 4 if "competition".
- Competition = Strava `workout_type` Race (runs/rides) OR `#competition` tag in the description (any sport, incl. swims).
- Day = `start_date_local`.
- Max 2 scoring activities per user per day (highest points).
- Points are recomputed per user per day on every create/update/delete (never accumulated incrementally).
- Season = name + start date + end date. Only activities inside the season score. Leaderboard is per season. Backfill covers the season range.

## Strava constraints
- New apps start in single-player mode (only the owner's Strava account can authenticate). Upgrade from the API Settings Dashboard raises capacity to 10 athletes (read limits 200 req/15min, 2000/day; overall 400/15min, 4000/day). Beyond 10 athletes, submit the app for review. Plan: develop in single-player mode, upgrade to 10 once there is something to test.
- Webhook payload contains only IDs/event type; fetch the activity with the user's token. Access tokens expire (6h), refresh needed.
- Rate limits (see above): no polling.
- Webhook delivery: Strava expects a 200 within ~2 s, retries a few times, then drops the event. Events sent during downtime are lost; recovered by reconciliation (#31).
- The activity list endpoint returns summaries without `description`; the `#competition` tag needs a detail fetch.
- Dev needs a public tunnel (cloudflared/ngrok) for webhooks.
- Strava terms restrict showing a user's data to other users; acceptable for a small private group.

## Stack
- Go (latest stable), VS Code with the official Go extension (gopls, delve), golangci-lint.
- HTTP: stdlib `net/http` + `chi` router.
- DB: SQLite (`modernc.org/sqlite`, pure Go) with plain `database/sql`; migrations with `goose`.
- OAuth: `golang.org/x/oauth2`. Sessions: `alexedwards/scs`.
- `log/slog`, config via env vars, stdlib `testing` (table-driven).
- Frontend: Vite + React + TypeScript, built and embedded in the Go binary via `go:embed`.
- Deploy: Docker multi-stage, Fly.io or cheap VPS. CI: GitHub Actions.

## Stretch features (learning value)
- Background worker (channel + goroutine) for webhook events, graceful shutdown with `context`.
- Backfill job with pagination and rate limiting (`golang.org/x/time/rate`).
- Token refresh via `http.RoundTripper` wrapper.
- Weekly/monthly leaderboards.
- Deauthorization handling (delete user data).

## Project state
- GitHub repo: mario595/leaderboard (private). Project: https://github.com/users/mario595/projects/6
- Tickets: issues #1-#24 and #31 created, grouped in milestones M0-M8 (M0 Setup, M1 Persistence, M2 Auth, M3 Scoring, M4 Strava client, M5 Webhooks, M6 API, M7 Frontend, M8 Backfill and deploy). Issue number order = suggested implementation order, except #31 (reconcile job) which goes right after #22. (#25-#30 are PRs.)
- Done: #1 (Strava app registered, single-player mode), #2 (Go + VS Code), #3 (skeleton), #4 (CI), #5 (SQLite, migrations, user store), #6 (seasons: table, store, `server season create|list` CLI), #7 (Strava OAuth login and callback), #8 (sessions, logout, auth middleware, /api/me), #9 (scoring domain: `internal/scoring`).
- In progress: #10 (activities table and store, per-day recompute service), branch `feature/10-activities-store`.
- Layout so far: `cmd/server/main.go`, `internal/config`, `internal/logging`, `internal/httpapi` (router, health, JSON helpers, middleware, /api/me), `internal/auth` (Strava OAuth, sessions, logout), `internal/testutil` (shared test assertions), `internal/scoring` (pure point rules: Discipline, Activity, Score), `internal/store` (db, migrate, embedded SQL migrations, users, seasons), `README.md`, `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml`, `.vscode/launch.json`.
- Local setup notes: Go 1.27.1 in `/usr/local/go`; `gcc` not installed yet, so `go test -race` does not work (needs cgo): `sudo apt install build-essential` before #14. `.env` holds `STRAVA_CLIENT_ID` and `STRAVA_CLIENT_SECRET` (git-ignored; `.env.example` lists names). Never read or print `.env`.

## Decisions log
- Router: chi chosen over Gin (net/http compatible, idiomatic, transferable).
- Module path: `github.com/mario595/leaderboard`. Packages under `internal/`, one binary under `cmd/server`.
- Config: `config.Load(getenv func(string) string)` (injected for testability); env vars `ADDR` (default `:8080`), `ENV` (`dev`|`prod`, default `dev`), `LOG_LEVEL` (`debug|info|warn|error`, default `info`). Case-sensitive. Reports only the first error (could use `errors.Join` later).
- Logging: `logging.New(w io.Writer, env, level string)`; text handler in dev, JSON in prod. Logger is passed explicitly, no global.
- HTTP: `NewRouter(logger) http.Handler`; middleware order RequestID, requestLogger, Recoverer (logger outside Recoverer so panics are logged as 500). Request log logs `r.URL.Path` only (query strings may contain OAuth codes).
- Server: `main` calls `run() error`; all four server timeouts set; graceful shutdown with a fresh 10s context.
- Tests: table-driven with `t.Run`, `httptest`, `bytes.Buffer` as log sink, test files named after the file under test.
- CI: GitHub Actions, `test` job (gofmt, vet, test) and `lint` job (golangci-lint v2.14, config in `.golangci.yml` with errorlint, noctx, sloglint, bodyclose, misspell, gofmt). Check configs with `golangci-lint config verify`.
- DB: SQLite via `modernc.org/sqlite` (driver name `sqlite`), `database/sql` pool limited to one connection, pragmas in the DSN (busy_timeout, WAL, foreign_keys), goose migrations embedded with `go:embed` and applied at startup. Times stored as unix-second INTEGERs; the store has an injectable `now` clock for tests. Relative `DB_PATH` resolves from the working directory (launch.json uses cwd = repo root). Tokens stored in plain text (MVP; encrypt later).
- Stretch idea: migrate the hand-written store to `sqlc` once there are several stores.
- Tests: mutation checks catch weak assertions (run them on a copy, never trust a test that has not failed once).
- Seasons (#6): dates stored as TEXT `YYYY-MM-DD`, inclusive bounds, overlap check in a transaction.
- CLI: one binary. `server` or `server serve` runs the web server; `server season create -name N -start YYYY-MM-DD -end YYYY-MM-DD` and `server season list` manage seasons; unknown commands exit with code 2. Shared setup in `openDatabase` (caller closes the DB). Command logic takes the store, args and an `io.Writer` so it is testable.
- Seasons: overlap check relies on SQLite's single writer (and the one-connection pool); on PostgreSQL it would need an exclusion constraint or SERIALIZABLE isolation with retry.
- Strava OAuth (#7): `golang.org/x/oauth2` has `endpoints.Strava`; request scopes `read` and `activity:read`; check the granted `scope` on the callback (users can untick scopes); state in a short-lived cookie; the `auth` package defines the `UserStore` interface it needs (consumer-defined). Never log tokens or the client secret.
- Strava OAuth (#7) is done: `auth.Handler` (Login, Callback), consumer-defined `UserStore` interface with a compile-time assertion in `auth.go`, the HTTP client with a 10 s timeout is injected, tests use a fake Strava (`httptest.NewServer`) and a fake store; error responses to users are generic and never include tokens or internal errors.
- Sessions (#8): `alexedwards/scs/v2` with `sqlite3store` (pure-Go compatible; needs a `sessions` table and has a cleanup goroutine to stop on shutdown), 30-day lifetime, cookie HttpOnly/Lax/Secure-in-prod, `RenewToken` on login, logout is a POST, typed context key for the user id, JSON DTOs (never encode `store.User`, it holds tokens), `NewRouter` takes a `Deps` struct. After login the browser is redirected to `/api/me` until the frontend exists.
- README.md at the root must be kept in sync (Status section) with each ticket.
- Done in #8: `NewRouter(Deps{Logger, Auth, Sessions, Users})`; `LoadAndSave` is applied once as a router middleware; ONE session manager per application, shared by the router and the auth handler (a middleware from one manager cannot serve another's handlers); `/api` is a chi `Route` with `requireAuth` (anonymous visitors get 401 even for unknown API paths); `requireAuth`/`userIDFromContext` live in `httpapi` with an unexported typed context key; the session key is the exported constant `auth.SessionUserIDKey`; the `/api/me` response is a DTO (never `store.User`); logout is POST-only and idempotent; shared test helpers live in `internal/testutil`; golangci-lint uses `run.relative-path-mode: wd` so VS Code can show findings.
- Scoring (#9): package `internal/scoring`, pure and season-agnostic (season filtering happens in the leaderboard query, #16). `Score([]Activity) []Award` handles any number of days, preserves input order, tie-break = start time then ID. Recompute, never accumulate: replays, duplicate events and reconciliation are safe because scoring is idempotent.
- Activity updates: webhook handler saves the event to a `webhook_events` table before returning 200 (#13); the worker treats the table as the queue and the channel as a wake-up signal, resuming pending events after restart (#14); an update rescores both the old and new day (#10); a reconcile job (startup + daily, recent window) fixes missed creates, updates and deletes (#31), reusing the backfill code (#22).
- Tests (#9): when a constant encodes an external fact (Strava codes) or a business rule (2/4 points, cap of 2), tests use the literal value, not the constant, so a wrong constant fails. To test a tie-break, order the input so the result is wrong without it. Testable examples (`ExampleX` with `// Output:`) document public APIs.
- Process lessons: wiring bugs (wrong manager in `main.go`, missing `Users` in `Deps`) are invisible to unit tests of single handlers, so keep at least one test per router wiring and always try the real flow in the browser; compile-time and lint checks do not catch `any` compared with a constant, discarded results, or missing `return` after `http.Error`.
