.PHONY: run build test vet lint

run:
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test -timeout 2m ./...

vet:
	go vet ./...

lint:
	golangci-lint run