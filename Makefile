# Birb-Nest Makefile

.PHONY: all build test clean run-api run-worker docker-build docker-push helm-verify

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
API_BINARY=birb-nest-api
WORKER_BINARY=birb-nest-worker

# Docker parameters
DOCKER_REGISTRY?=birbparty
DOCKER_TAG?=latest

all: test build

build: build-api build-worker

build-api:
	$(GOBUILD) -o $(API_BINARY) -v ./cmd/api

build-worker:
	$(GOBUILD) -o $(WORKER_BINARY) -v ./cmd/worker

test:
	$(GOTEST) -v ./...

test-integration:
	$(GOTEST) -v ./tests/integration/... -tags=integration

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	$(GOCLEAN)
	rm -f $(API_BINARY)
	rm -f $(WORKER_BINARY)
	rm -f coverage.out coverage.html

run-api: build-api
	./$(API_BINARY)

run-worker: build-worker
	./$(WORKER_BINARY)

# Development with hot reload
dev-api:
	air -c .air.toml

# Docker commands
docker-build: docker-build-api docker-build-worker

docker-build-api:
	docker build -f Dockerfile.api -t $(DOCKER_REGISTRY)/$(API_BINARY):$(DOCKER_TAG) .

docker-build-worker:
	docker build -f Dockerfile.worker -t $(DOCKER_REGISTRY)/$(WORKER_BINARY):$(DOCKER_TAG) .

docker-push: docker-push-api docker-push-worker

docker-push-api:
	docker push $(DOCKER_REGISTRY)/$(API_BINARY):$(DOCKER_TAG)

docker-push-worker:
	docker push $(DOCKER_REGISTRY)/$(WORKER_BINARY):$(DOCKER_TAG)

# Helm chart commands
helm-verify:
	@echo "🔍 Running Helm chart verification..."
	@chmod +x scripts/verify-helm-chart.sh
	@./scripts/verify-helm-chart.sh

helm-lint:
	@if command -v helm > /dev/null 2>&1; then \
		echo "🔍 Running Helm lint..."; \
		helm lint charts/birb-nest; \
		helm lint charts/birb-nest -f charts/birb-nest/values-replica-example.yaml; \
	else \
		echo "⚠️  Helm not installed, running basic verification..."; \
		make helm-verify; \
	fi

helm-template-primary:
	@if command -v helm > /dev/null 2>&1; then \
		helm template test-primary charts/birb-nest; \
	else \
		echo "❌ Helm not installed"; \
		exit 1; \
	fi

helm-template-replica:
	@if command -v helm > /dev/null 2>&1; then \
		helm template test-replica charts/birb-nest -f charts/birb-nest/values-replica-example.yaml; \
	else \
		echo "❌ Helm not installed"; \
		exit 1; \
	fi

# Database operations
db-migrate:
	@echo "Running database migrations..."
	@psql -h localhost -U postgres -d birbnest -f scripts/init-db.sql

# Local development setup
setup-local:
	@echo "Setting up local development environment..."
	docker-compose -f docker-compose.dev.yml up -d
	@echo "Waiting for services to start..."
	@sleep 5
	make db-migrate

teardown-local:
	docker-compose -f docker-compose.dev.yml down -v

# CI/CD helpers
ci-test: helm-verify test test-integration

ci-build: build docker-build

# Dependency management
deps:
	$(GOMOD) download
	$(GOMOD) tidy

deps-update:
	$(GOMOD) tidy
	$(GOGET) -u ./...

# Code quality
lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

fmt:
	go fmt ./...

vet:
	go vet ./...

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Run tests and build"
	@echo "  build            - Build API and worker binaries"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-coverage    - Generate test coverage report"
	@echo "  clean            - Clean build artifacts"
	@echo "  run-api          - Build and run API"
	@echo "  run-worker       - Build and run worker"
	@echo "  dev-api          - Run API with hot reload"
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-push      - Push Docker images"
	@echo "  helm-verify      - Verify Helm chart"
	@echo "  helm-lint        - Lint Helm chart (requires Helm)"
	@echo "  setup-local      - Setup local development environment"
	@echo "  teardown-local   - Teardown local development environment"
	@echo "  deps             - Download dependencies"
	@echo "  lint             - Run linter"
	@echo "  fmt              - Format code"

.DEFAULT_GOAL := help
