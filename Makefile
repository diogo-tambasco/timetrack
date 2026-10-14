# Makefile do Timetrack — `make check-full` é o gate oficial de fim de fase
# (CA-18 / TDD.md §6).
COMPOSE := docker compose -f docker-compose.dev.yml
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/timetrack_test?sslmode=disable

.PHONY: dev up down migrate test-back test-front test-e2e build build-front build-back check check-full

# ---- dev local (Go + Vite em watch) ----
dev:
	@echo "subindo backend (:8080) + vite dev server (:5173)..."
	@trap 'kill 0' EXIT; \
	( cd backend && go run ./cmd/server ) & \
	( cd frontend && npm run dev ) & \
	wait

# ---- compose local ----
up:
	$(COMPOSE) up -d --build timetrack-db timetrack-app

down:
	$(COMPOSE) down

# atalho de inspeção — as migrações rodam no boot do app (make up)
migrate:
	$(COMPOSE) exec timetrack-db psql -U timetrack -d timetrack -c 'TABLE schema_migrations;'

# ---- testes ----
# sobe o Postgres efêmero de teste ANTES do go test — sem skip silencioso
test-back:
	$(COMPOSE) --profile test up -d --wait timetrack-db-test
	cd backend && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test ./...

test-front:
	cd frontend && npx vitest run

# o backend do E2E (playwright.config) usa o Postgres de teste da 5433
test-e2e:
	$(COMPOSE) --profile test up -d --wait timetrack-db-test
	cd frontend && npx playwright test

# ---- builds ----
build-front:
	cd frontend && npm run build

build-back:
	cd backend && go build -o bin/timetrack ./cmd/server

build: build-front build-back

# ---- gates ----
# check (CA-18): testes Go + Vitest + Playwright + os dois builds
# (+ scripts de verificação da fase 00: segredos, cores, fontes, pwa)
check: test-back test-front build
	./scripts/check-segredos.sh
	./scripts/check-cores.sh
	./scripts/check-fonts.sh
	./scripts/check-pwa.sh
	$(MAKE) test-e2e
	@echo "check: tudo verde"

# gate oficial de fim de fase: check + smoke do compose (CA-15)
check-full: check
	./scripts/smoke-compose.sh
	@echo "check-full: GATE VERDE"
