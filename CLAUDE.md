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

## Spec format
- Written like a senior engineer handing work to a junior: "create file X with type Y", "add method Z to X", "modify file X to include Y".
- Very detailed at first. Detail level is reduced when Mario asks.
- Follow real-world Go conventions and good practices (project layout, naming, error handling, testing, idiomatic style).
- Where Go differs from Java/Python/Django, call out the difference and the reason.

## Rules for Claude
- No code written for Mario unless told otherwise. Small illustrative snippets only if explicitly requested.
- Reviews: point out idiomatic issues, conventions, pitfalls, and why.
- Keep communication direct and brief (see global CLAUDE.md).
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
- New API apps are limited to 1 athlete until Strava approves a capacity increase (request early).
- Webhook payload contains only IDs/event type; fetch the activity with the user's token. Access tokens expire (6h), refresh needed.
- Rate limits (~100 req/15 min, ~1000/day by default): no polling.
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
- Tickets: issues #1-#24 created, grouped in milestones M0-M8 (M0 Setup, M1 Persistence, M2 Auth, M3 Scoring, M4 Strava client, M5 Webhooks, M6 API, M7 Frontend, M8 Backfill and deploy). Issue number order = suggested implementation order.

## Decisions log
- Router: chi chosen over Gin (net/http compatible, idiomatic, transferable).
