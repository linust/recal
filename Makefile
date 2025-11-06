.PHONY: help build build-local test test-local test-coverage test-integration run clean docker-build docker-run docker-clean fmt vet lint ci-test test-ci-local watch-ci

# Docker image settings
DOCKER_IMAGE := recal
DOCKER_TAG := latest
BUILDER_IMAGE := golang:1.21-alpine

# Binary name and location
BINARY := bin/recal

# Default target
help:
	@echo "Available targets:"
	@echo "  build           - Build the binary using Docker (reproducible)"
	@echo "  build-local     - Build the binary using local Go (faster, less reproducible)"
	@echo "  test            - Run all tests using Docker (reproducible)"
	@echo "  test-local      - Run all tests using local Go (faster)"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-integration - Run integration tests against live server"
	@echo "  test-ci-local   - Run CI integration tests locally (before pushing)"
	@echo "  run             - Run the application using Docker"
	@echo "  clean           - Remove build artifacts"
	@echo "  docker-build    - Build Docker image"
	@echo "  docker-run      - Run Docker container"
	@echo "  docker-clean    - Remove Docker images"
	@echo "  fmt             - Format code using Docker"
	@echo "  vet             - Run go vet using Docker"
	@echo "  lint            - Run golangci-lint using Docker"
	@echo "  ci-test         - Run CI test suite (used by GitHub Actions)"
	@echo "  watch-ci        - Watch CI build and auto-download logs on failure"
	@echo "  dev             - Quick dev cycle: test-local + build-local"

# Build the binary using Docker for reproducibility
build:
	@echo "Building binary using Docker (reproducible build)..."
	@mkdir -p bin
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'apk add --no-cache git && go build -ldflags="-w -s" -trimpath -o $(BINARY) ./cmd/recal'
	@echo "Build complete: ./$(BINARY)"

# Build locally (faster but less reproducible)
build-local:
	@echo "Building binary using local Go..."
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/recal

# Run tests using Docker
test:
	@echo "Running all tests using Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'apk add --no-cache git && go test -v ./...'
	@echo ""
	@echo "✓ All unit and integration tests passed!"
	@echo ""
	@echo "Test summary:"
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'go test ./... 2>&1 | grep -E "^(ok|FAIL|\?)"'

# Run tests locally (faster)
test-local:
	@echo "Running all tests locally..."
	go test -v ./...
	@echo ""
	@echo "✓ All unit and integration tests passed!"
	@echo ""
	@echo "Test summary:"
	@go test ./... 2>&1 | grep -E "^(ok|FAIL|\?)"

# Run the application using Docker
run: docker-build
	@echo "Starting application in Docker..."
	docker run --rm -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f ical-filter ical-proxy-filter recal
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) 2>/dev/null || true
	docker rmi $(DOCKER_IMAGE):builder 2>/dev/null || true

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 --name $(DOCKER_IMAGE) $(DOCKER_IMAGE):$(DOCKER_TAG)

# Remove Docker images
docker-clean:
	@echo "Removing Docker images..."
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true

# Format code using Docker
fmt:
	@echo "Formatting code using Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'go fmt ./...'

# Run go vet using Docker
vet:
	@echo "Running go vet using Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'go vet ./...'

# Run golangci-lint using Docker
lint:
	@echo "Running golangci-lint using Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		golangci/golangci-lint:v1.55-alpine \
		golangci-lint run -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage using Docker..."
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(BUILDER_IMAGE) \
		sh -c 'apk add --no-cache git && go test -v -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

# Run integration tests against a live server
# Usage: make test-integration (assumes server running on localhost:8080)
# Or:    make test-integration BASE_URL=http://myserver:8080
test-integration:
	@echo "Running integration tests..."
	@if [ ! -f ./test-server.sh ]; then \
		echo "Error: test-server.sh not found"; \
		exit 1; \
	fi
	@chmod +x ./test-server.sh
	@./test-server.sh $(BASE_URL)

# CI test target (for GitHub Actions)
ci-test: test
	@echo ""
	@echo "✓ CI tests passed!"

# Development helpers
.PHONY: dev-setup
dev-setup:
	@echo "Setting up development environment..."
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@echo "Development environment ready!"

# Quick dev cycle: test + build
.PHONY: dev
dev: test-local build-local
	@echo "Development build complete!"

# Run local CI integration tests (mimics GitHub Actions)
test-ci-local:
	@echo "Running local CI integration tests..."
	@./test-local.sh

# Watch GitHub Actions CI and auto-download logs on failure
watch-ci:
	@./watch-ci.sh
