.PHONY: run build test vet lint web-install web-dev web-test web-check web-build strava-subscription strava-subscribe strava-unsubscribe docker-build docker-run

# Exports the variables from .env (if present) into the recipe's shell.
# Make runs each recipe line in its own shell, so use it on the same line.
LOAD_ENV = set -a; if [ -f .env ]; then . ./.env; fi; set +a

STRAVA_SUBSCRIPTIONS = https://www.strava.com/api/v3/push_subscriptions

run:
	$(LOAD_ENV); go run ./cmd/server

# The binary embeds the frontend (internal/spa), so build that first.
build: web-build
	go build -o bin/server ./cmd/server

test:
	go test -race -timeout 5m ./...

vet:
	go vet ./...

lint:
	golangci-lint run

# Frontend (web/). Node version: web/.nvmrc.

# Install exactly what package-lock.json lists.
web-install:
	cd web && npm ci

# Vite dev server on :5173, proxying /api and /auth to the Go server on :8080.
# Run the server with BASE_URL=http://localhost:5173 so the Strava login
# returns through the proxy.
web-dev:
	cd web && npm run dev

web-test:
	cd web && npm test

# Everything CI checks on the frontend.
web-check:
	cd web && npm run lint && npm run typecheck && npm run format:check && npm test && npm run build

web-build:
	cd web && npm run build

# Strava webhook subscription (one per Strava application). Recipes start
# with @ so make does not echo commands that reference the client secret.

# Show the current subscription, if any.
strava-subscription:
	@$(LOAD_ENV); curl -sS -G $(STRAVA_SUBSCRIPTIONS) \
		-d client_id="$$STRAVA_CLIENT_ID" -d client_secret="$$STRAVA_CLIENT_SECRET"; echo

# Create the subscription. The server must be reachable at CALLBACK: Strava
# calls it during this request to check STRAVA_VERIFY_TOKEN.
strava-subscribe:
	@test -n "$(CALLBACK)" || { echo "usage: make strava-subscribe CALLBACK=https://<host>/webhook/strava"; exit 2; }
	@$(LOAD_ENV); curl -sS -X POST $(STRAVA_SUBSCRIPTIONS) \
		-F client_id="$$STRAVA_CLIENT_ID" -F client_secret="$$STRAVA_CLIENT_SECRET" \
		-F callback_url="$(CALLBACK)" -F verify_token="$$STRAVA_VERIFY_TOKEN"; echo

# Delete the subscription with the given ID (Strava answers 204 on success).
strava-unsubscribe:
	@test -n "$(ID)" || { echo "usage: make strava-unsubscribe ID=<subscription id>"; exit 2; }
	@$(LOAD_ENV); curl -sS -X DELETE "$(STRAVA_SUBSCRIPTIONS)/$(ID)" -w "HTTP %{http_code}\n" \
		-F client_id="$$STRAVA_CLIENT_ID" -F client_secret="$$STRAVA_CLIENT_SECRET"

# Docker image: the frontend and a static binary on distroless (see Dockerfile).
docker-build:
	docker build -t leaderboard .

# Runs the image with the settings from .env. Docker reads .env literally: no
# quotes, no "export". BASE_URL is overridden because .env points to the Vite
# dev server (:5173). The database lives in the leaderboard-data volume, which
# outlives the container.
docker-run:
	docker run --rm --name leaderboard -p 8080:8080 --env-file .env \
		-e BASE_URL=http://localhost:8080 -v leaderboard-data:/data leaderboard
