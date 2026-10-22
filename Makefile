.PHONY: run build test vet lint strava-subscription strava-subscribe strava-unsubscribe

# Exports the variables from .env (if present) into the recipe's shell.
# Make runs each recipe line in its own shell, so use it on the same line.
LOAD_ENV = set -a; if [ -f .env ]; then . ./.env; fi; set +a

STRAVA_SUBSCRIPTIONS = https://www.strava.com/api/v3/push_subscriptions

run:
	$(LOAD_ENV); go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test -race -timeout 5m ./...

vet:
	go vet ./...

lint:
	golangci-lint run

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
