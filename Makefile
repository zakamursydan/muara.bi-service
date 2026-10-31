# muara.bi — unified build, test and run targets.
# Images are CGO/DuckDB builds, so `make up` compiles the backend inside the
# container; local `run-*` targets use the toolchain on your PATH instead.

COMPOSE ?= docker compose
BACKEND := backend
FRONTEND := frontend

.PHONY: help up down rebuild ps logs test vet lint tidy \
        api-build worker-build run-api run-worker \
        image-core image-web web-install web-dev web-build

help:
	@grep -hE '^[a-z-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

# ---------------------------------- stack ------------------------------------
up: ## build images and start the full compose stack
	$(COMPOSE) up -d --build

down: ## stop the stack (volumes are kept)
	$(COMPOSE) down

rebuild: ## force-rebuild images and restart
	$(COMPOSE) up -d --build --force-recreate

ps: ## show container status and health
	$(COMPOSE) ps

logs: ## follow API and worker logs
	$(COMPOSE) logs -f api worker

# --------------------------------- backend -----------------------------------
test: ## run backend unit tests
	cd $(BACKEND) && go test ./...

vet: ## run go vet
	cd $(BACKEND) && go vet ./...

tidy: ## sync go.mod / go.sum
	cd $(BACKEND) && go mod tidy

api-build: ## compile the API binary into backend/bin
	cd $(BACKEND) && go build -o bin/api ./cmd/api

worker-build: ## compile the worker binary into backend/bin
	cd $(BACKEND) && go build -o bin/worker ./cmd/worker

run-api: api-build ## run the API locally against backend/.env
	cd $(BACKEND) && ./bin/api

run-worker: worker-build ## run the Asynq worker locally
	cd $(BACKEND) && ./bin/worker

image-core: ## build only the backend image
	docker build -f Dockerfile -t muara-bi-core:dev .

image-web: ## build only the web image
	docker build -f $(FRONTEND)/Dockerfile -t muara-bi-web:dev $(FRONTEND)

# --------------------------------- frontend ----------------------------------
web-install: ## install frontend dependencies
	cd $(FRONTEND) && npm ci

lint: ## run eslint
	cd $(FRONTEND) && npm run lint -- .

web-dev: ## start the Next dev server (proxies /api to localhost:8080)
	cd $(FRONTEND) && npm run dev

web-build: ## type-check and build the frontend
	cd $(FRONTEND) && npm run build
