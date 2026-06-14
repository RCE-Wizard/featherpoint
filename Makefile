.PHONY: all server agent test migrate-up migrate-down dev-db dev-db-stop seed gen-certs

MODULE := github.com/featherpoint/swinv

all: server agent

server:
	CGO_ENABLED=0 go build -o bin/swinv-server ./cmd/server

agent:
	CGO_ENABLED=0 go build -o bin/swinv-agent ./cmd/agent

test:
	go test ./...

dev-db:
	docker compose -f deploy/docker-compose.yml up -d

dev-db-stop:
	docker compose -f deploy/docker-compose.yml down

migrate-up:
	@which migrate > /dev/null || (echo "Install golang-migrate: https://github.com/golang-migrate/migrate" && exit 1)
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

# Quick DB setup for dev (uses default dev creds)
dev-setup: dev-db
	@echo "Waiting for postgres..."
	@sleep 3
	DATABASE_URL=postgres://swinv:swinv_dev@localhost:5432/swinv $(MAKE) migrate-up

seed:
	go run scripts/seed/main.go

gen-certs:
	go run scripts/gen-dev-certs/main.go
