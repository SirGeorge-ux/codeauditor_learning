.PHONY: help test lint build dev clean validate fix check

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# --- Backend ---

test-backend: ## Run Go tests
	cd backend && go test -count=1 ./internal/...

lint-backend: ## Lint Go code (requires golangci-lint)
	cd backend && golangci-lint run ./...

build-backend: ## Build Go binary
	cd backend && go build -o api ./cmd/api/

dev-backend: ## Run backend in dev mode
	cd backend && go run ./cmd/api/

# --- Frontend ---

test-frontend: ## Run Angular tests
	cd frontend/codeauditor && npx ng test --watch=false

lint-frontend: ## Lint Angular code
	cd frontend/codeauditor && npx eslint src/

build-frontend: ## Build Angular app
	cd frontend/codeauditor && npx ng build

dev-frontend: ## Run Angular dev server
	cd frontend/codeauditor && pnpm start

e2e: ## Run Playwright E2E tests
	cd frontend/codeauditor && pnpm e2e

# --- Full stack ---

test: test-backend test-frontend ## Run all tests

ci: lint test build ## Full CI pipeline (lint + test + build)

lint: lint-backend lint-frontend ## Lint all code

build: build-backend build-frontend ## Build all

dev: ## Start full dev stack (Docker + backend + frontend)
	docker compose up -d
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:4200"
	@echo "PGAdmin: http://localhost:3000"

clean: ## Clean build artifacts
	rm -rf backend/api backend/bin/
	rm -rf frontend/codeauditor/dist/ frontend/codeauditor/.angular/

# --- Quality gates ---

validate: validate-backend validate-frontend ## Validate code quality (pre-deploy gate)

check: validate ## Alias for validate

validate-backend: ## Validate Go code (gofumpt + golangci-lint)
	@echo "==> Validating Backend (Go)..."
	@cd backend && test -z "$$(gofumpt -l .)" || (echo "  gofumpt: files need formatting. Run 'make fix-backend'" && exit 1)
	@cd backend && golangci-lint run
	@echo "==> Backend OK"

validate-frontend: ## Validate TypeScript code (Prettier + ESLint)
	@echo "==> Validating Frontend (TypeScript)..."
	@cd frontend/codeauditor && pnpm exec prettier --check "src/**/*.{ts,html,css}"
	@cd frontend/codeauditor && pnpm lint
	@echo "==> Frontend OK"

fix: fix-backend fix-frontend ## Fix formatting across the project

fix-backend: ## Fix Go formatting (gofumpt)
	@echo "==> Fixing Backend (Go)..."
	@cd backend && gofumpt -w .
	@echo "==> Backend fixed"

fix-frontend: ## Fix TypeScript formatting (Prettier)
	@echo "==> Fixing Frontend (TypeScript)..."
	@cd frontend/codeauditor && pnpm format
	@echo "==> Frontend fixed"
