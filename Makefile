.PHONY: run build test vet

run:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

vet:
	go vet ./...