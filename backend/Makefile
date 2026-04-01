.PHONY: build test dev lint docker-up docker-down migrate

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -v -count=1

dev:
	go run ./cmd/server

lint:
	go vet ./...

docker-up:
	docker compose -f ../docker-compose.yml up -d

docker-down:
	docker compose -f ../docker-compose.yml down

migrate:
	@echo "Migrations run automatically on server startup."
	@echo "To run them standalone, start Postgres and run: go run ./cmd/server"
