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
- Tickets: issues #1-#24 and #31 created, grouped in milestones M0-M8 (M0 Setup, M1 Persistence, M2 Auth, M3 Scoring, M4 Strava client, M5 Webhooks, M6 API, M7 Frontend, M8 Backfill and deploy). Issue number order = suggested implementation order, except #31 (reconcile job) which goes right after #22, and #36 (invitation links, M2) which goes any time before #24 (Deploy). (#25-#30 are PRs.)
- Done: #1 (Strava app registered, single-player mode), #2 (Go + VS Code), #3 (skeleton), #4 (CI), #5 (SQLite, migrations, user store), #6 (seasons: table, store, `server season create|list` CLI), #7 (Strava OAuth login and callback), #8 (sessions, logout, auth middleware, /api/me), #9 (scoring domain: `internal/scoring`), #10 (activities table, store and per-day recompute service), #11 (Strava API client with token refresh), #12 (fetch an activity, typed errors), #13 (webhook handshake and event receiver), #14 (background worker), #15 (deauthorization).
- In progress: #16 (GET /api/leaderboard), branch `feature/16-leaderboard-api`.
- Layout so far: `cmd/server/main.go`, `internal/config`, `internal/logging`, `internal/httpapi` (router, health, JSON helpers, middleware, /api/me), `internal/auth` (Strava OAuth, sessions, logout), `internal/testutil` (shared test assertions), `internal/scoring` (pure point rules: Discipline, Activity, Score), `internal/activity` (Service: Save/Delete with per-day recompute in one transaction), `internal/strava` (API, per-user Client, token refresh and save, Activity, typed errors), `internal/webhook` (Verify, Receive), `internal/worker` (event worker: process, retry policy, Run loop), `internal/store` (db, migrate, embedded SQL migrations, shared declarations in store.go, users, seasons, activities), `README.md`, `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml`, `.vscode/launch.json`.
- Local setup notes: Go 1.27.1 in `/usr/local/go`; `gcc` installed (#11), so `go test -race` works; CI and `make test` run with `-race`. `.env` holds `STRAVA_CLIENT_ID` and `STRAVA_CLIENT_SECRET` (git-ignored; `.env.example` lists names). Never read or print `.env`.

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
- Activities (#10): `strava_id` is the primary key; `start_local` stored as TEXT `YYYY-MM-DDTHH:MM:SS` (wall clock, no zone), `day` column derived by the store and indexed with `user_id`; `points` written only by the recompute (`SetPoints`), never by `Upsert`; `ON DELETE CASCADE` from users. Stores take a `DBTX` (shared by `*sql.DB` and `*sql.Tx`) and expose `WithTx(tx)` returning a copy. `activity.Service` reads the old row before `Upsert` to recompute the old day when an edit moves an activity; `Delete` of an unknown ID is a no-op.
- One-connection pool pitfall: inside a transaction, a query through the plain store (not `WithTx`) deadlocks. Tests run with `-timeout 2m` (CI and Makefile) so this fails fast. Linters `rowserrcheck` and `sqlclosecheck` catch missing `rows.Err()`/`rows.Close()`.
- Go pitfall seen twice in #10: a zero-value struct is not "missing" (`Day()` of a zero Activity is `0001-01-01`); track absence with a separate `found` bool.
- Process: TDD by pairing works well (Claude writes one failing test, Mario implements). Tests that pass immediately pin behaviour; prove they can fail with a mutation.
- Strava client (#11): `strava.API` (shared: oauth2 config, base `*http.Client` with timeout, `TokenStore`, base URL) builds a per-user `Client` with `ForUser(userID, *oauth2.Token)`. Chain: `oauth2.Transport` -> `savingTokenSource` (our decorator, mutex + `last`, saves each new token once, a failed save fails the request) -> `cfg.TokenSource` (library refresh). Pitfalls avoided: build the `http.Client` ourselves (`oauth2.NewClient` drops the timeout); the TokenSource context must be long-lived and carry our client via `oauth2.HTTPClient` (it is reused for every refresh). `strava` does not import `store` (consumer-defined `TokenStore`; compile-time check in a test file). `UserStore.UpdateTokens` returns `ErrNotFound` for unknown users. Generic `get` only; endpoints and typed errors are #12.
- Strava activity (#12): `activityDTO` (unexported, JSON tags, `StartDateLocal` string, `MovingTime` int seconds) -> `strava.Activity` (exported, plain Go types, no user ID) -> `store.Activity` built by the worker (#14) with the user ID. Use `sport_type`, never the legacy `type`; use `start_date_local` parsed with RFC3339 (UTC location = wall clock), never `start_date`. JSON null into a non-pointer leaves the zero value, so `workout_type`/`description` need no pointers. Unknown fields ignored (no `DisallowUnknownFields`). Fixtures in `internal/strava/testdata`.
- Strava errors: `ErrNotFound` (404), `ErrUnauthorized` (401, or refresh rejected with 400/401, wrapped together with the `*oauth2.RetrieveError` via two `%w`), `*RateLimitError` (429, usage/limits parsed from `X-RateLimit-Usage`/`X-RateLimit-Limit`, zeros when unparseable). A 500 from the token endpoint is NOT unauthorized (transient).
- Webhooks (#13): `GET`/`POST /webhook/strava`, public. `Verify` echoes `hub.challenge` only for mode `subscribe` and a matching `STRAVA_VERIFY_TOKEN` (`crypto/subtle`). `Receive`: `MaxBytesReader` 4 KiB, subscription ID must equal `STRAVA_SUBSCRIPTION_ID` (unset = reject all), validated types and positive IDs, insert into `webhook_events` (unique on object_type, object_id, aspect_type, event_time; `ON CONFLICT DO NOTHING`, duplicates answer 200 without waking), raw `updates` JSON (`"{}"` when absent or null), wake via `select` with `default` on a `chan<- struct{}` (nil until #14), 500 on store errors so Strava retries. Handler does no user lookup and no Strava calls. Strava does not sign events: #14 must confirm deletes with a GET returning `ErrNotFound`.
- Subscription management: `make strava-subscription`, `make strava-subscribe CALLBACK=...`, `make strava-unsubscribe ID=...` (curl, `.env` loaded, recipes prefixed with `@` to hide the secret).
- Dev tunnel: named Cloudflare tunnel `leaderboard-dev` -> `https://leaderboard-dev.lospalmasnumeroseis.site` (Mario's unrelated domain, dev only), config in `/etc/cloudflared/config.yml` (copy of `~/.cloudflared/config.yml`), runs as the `cloudflared` systemd service. Subscription for the current (dev) Strava app points there.
- Open question for #24: production domain (not lospalmasnumeroseis.site) and whether to use a second Strava app (one app per Strava account, I believe: needs a second account; keeps dev webhooks working) or move the single app's subscription at go-live.
- Dev VM clock: the VM (Apple host) lost 46 minutes after the host slept; chrony now steps the clock (`makestep 1 -1` in `/etc/chrony/conf.d/makestep.conf`). A slow clock makes Strava reject tokens we think are valid.
- CI and `make test` use `-race -timeout 5m` (the race suite took ~1m25s at #12).
- Worker (#14): one goroutine, oldest due event first (`EventStore.NextDue` with `next_attempt_at`, migration 0006; never edit an applied migration). Every activity event reconciles with Strava: found -> `activity.Service.Save`, `ErrNotFound` -> `Delete`, whatever the aspect. Athlete events and non-users -> done (#15 handles deauthorization). Retry: rate limit -> next 15-min window or UTC midnight (not counted against the limit), `ErrUnauthorized` -> failed at once (no forced refresh: decided against, clock skew was a dev-VM issue), other -> backoff 1 min doubling to 1 h, `maxAttempts = 8`. `Run` drains, then `select` on `ctx.Done()`, wake, `time.After(pollInterval)`; checks `ctx.Err()` between events and runs each event with `context.WithoutCancel` so shutdown never cuts one in half. Client factory is a `ClientFunc` (no covariant returns: `ForUser` returns `*strava.Client`, wrapped in a closure in main). main: own context for the worker, `sync.WaitGroup.Go`, a defer that cancels and waits (runs before `db.Close`).
- Wiring bug avoided in #14: `strava.New` must get `strava.DefaultBaseURL`, not `cfg.BaseURL` (our own URL would 404 and the worker would delete every activity).
- Dev VM network: the VM's bridged network loses ~35% of packets (the Mac host has 0%), so the dev tunnel keeps reconnecting and Strava webhooks fail intermittently. Tunnel uses `protocol: http2`. Live webhook testing is deferred to the deploy (#24); locally, POST a fake event to `localhost:8080/webhook/strava`. Fix later: switch the VM to NAT networking.
- Deauthorization (#15): `UserStore.DeleteByStravaID` deletes the athlete's `webhook_events` and the user row (activities via cascade) in one transaction, one statement per `ExecContext` (a multi-statement Exec reports `RowsAffected` of the last statement only; modernc does not check argument counts, so extra args pass silently). The worker handles athlete events before any user lookup, so non-users' events are deleted too; deletes only on `updates.authorized == "false"` (a string), logs and skips undecodable updates. `requireAuth(sessions, users, logger)` loads the session's user and stores the `store.User` in the context (`userFromContext`); a deleted user -> `sessions.Destroy` before writing the response (headers go out with the first body byte), 401 on every `/api` route; store error -> 500, session kept. `meHandler` reads the user from the context. scs session rows cannot be found by user (encoded blob): they expire on their own. Other members' points need no recompute (scores are per user and day).
- Concurrency bugs are intermittent: a missing lock failed 1 of 3 `-race` runs. Run concurrent tests with `-race` (now in CI and `make test`) and `-count` when checking a fix.
- Process lessons: wiring bugs (wrong manager in `main.go`, missing `Users` in `Deps`) are invisible to unit tests of single handlers, so keep at least one test per router wiring and always try the real flow in the browser; compile-time and lint checks do not catch `any` compared with a constant, discarded results, or missing `return` after `http.Error`.
