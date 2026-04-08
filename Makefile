.PHONY: dev dev-api dev-web dev-agent build test migrate lint clean help

# Default target
help:
	@echo "BlueSend Development Commands"
	@echo "================================"
	@echo "  make dev          Start full stack with Docker Compose"
	@echo "  make dev-api      Run Go API in development mode (hot reload)"
	@echo "  make dev-web      Run Next.js frontend in development mode"
	@echo "  make dev-agent    Run device agent in development mode"
	@echo "  make build        Build all production artifacts"
	@echo "  make test         Run all tests"
	@echo "  make migrate      Run database migrations"
	@echo "  make lint         Lint all code"
	@echo "  make clean        Remove build artifacts"
	@echo "  make device-register  Register a new physical device"

# Start full development stack
dev:
	@cp -n .env.example .env 2>/dev/null || true
	docker compose up --build

# API development (requires local postgres + redis)
dev-api:
	cd services/api && air

# Frontend development
dev-web:
	cd apps/web && npm run dev

# Device agent development
dev-agent:
	cd apps/device-agent && go run main.go

# Build all services
build: build-api build-agent build-web

build-api:
	cd services/api && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o ../../dist/api ./cmd/...

build-agent-mac:
	cd apps/device-agent && GOOS=darwin GOARCH=arm64 go build -o ../../dist/bluesend-agent-arm64 .
	cd apps/device-agent && GOOS=darwin GOARCH=amd64 go build -o ../../dist/bluesend-agent-amd64 .

build-web:
	cd apps/web && npm run build

# Database migrations
migrate:
	@echo "Running migrations..."
	docker compose exec postgres psql -U bluesend -d bluesend -f /docker-entrypoint-initdb.d/001_initial.up.sql
	docker compose exec postgres psql -U bluesend -d bluesend -f /docker-entrypoint-initdb.d/002_ghl.up.sql
	docker compose exec postgres psql -U bluesend -d bluesend -f /docker-entrypoint-initdb.d/003_billing.up.sql

migrate-down:
	docker compose exec postgres psql -U bluesend -d bluesend -f /docker-entrypoint-initdb.d/001_initial.down.sql

# Tests
test: test-api test-web

test-api:
	cd services/api && go test ./... -v -race -coverprofile=coverage.out

test-web:
	cd apps/web && npm test

# Linting
lint: lint-api lint-web

lint-api:
	cd services/api && golangci-lint run ./...

lint-web:
	cd apps/web && npm run lint

# Register a physical device
device-register:
	@read -p "Device name: " name; \
	read -p "Device type (mac_mini/iphone): " type; \
	curl -X POST ${API_URL:-http://localhost:8080}/api/admin/devices/register \
		-H "X-Admin-Key: ${ADMIN_API_KEY}" \
		-H "Content-Type: application/json" \
		-d "{\"name\": \"$$name\", \"type\": \"$$type\"}"

# Clean build artifacts
clean:
	rm -rf dist/
	cd apps/web && rm -rf .next/
	cd services/api && rm -f coverage.out
