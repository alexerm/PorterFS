# PorterFS Makefile

.PHONY: build test clean docker run help

# Variables
BINARY_NAME=porter
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd/porter

build-all: ## Build binaries for all platforms
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}-darwin-amd64 ./cmd/porter
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BINARY_NAME}-darwin-arm64 ./cmd/porter
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}-linux-amd64 ./cmd/porter
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BINARY_NAME}-linux-arm64 ./cmd/porter

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and show coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration: build ## Run integration tests
	@echo "Running integration tests..."
	./scripts/test-connection.sh

test-ssl: build ## Run SSL integration tests
	@echo "Running SSL integration tests..."
	./scripts/test-ssl.sh

test-simple: build ## Run simple tests
	@echo "Running simple tests..."
	./scripts/test/test-simple.sh

test-comprehensive: build ## Run comprehensive tests
	@echo "Running comprehensive tests..."
	./scripts/test/test-comprehensive.sh

clean: ## Clean build artifacts
	rm -f ${BINARY_NAME}*
	rm -f test-client
	rm -f coverage.out coverage.html
	rm -rf .multipart
	@echo "Note: data/ and data-ssl/ directories are gitignored and not cleaned"

docker: ## Build Docker image
	docker build -t porterfs/porter:${VERSION} .

docker-run: docker ## Build and run Docker container
	@mkdir -p data 2>/dev/null || true
	docker run -p 9000:9000 -v $(PWD)/data:/data porterfs/porter:${VERSION}

run: build ## Build and run the server
	./$(BINARY_NAME) -config config.yaml

run-ssl: build ## Build and run the server with SSL
	./$(BINARY_NAME) -config config-ssl.yaml

dev: ## Run in development mode with auto-reload
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

lint: ## Run linter
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

fmt: ## Format code
	go fmt ./...
	go mod tidy

deps: ## Download dependencies
	go mod download
	go mod verify

install: build ## Install binary to GOPATH/bin
	go install ${LDFLAGS} ./cmd/porter

uninstall: ## Remove binary from GOPATH/bin
	rm -f $(shell go env GOPATH)/bin/$(BINARY_NAME)

# Version targets
version: ## Show version information
	@echo "Version: ${VERSION}"
	@echo "Build Time: ${BUILD_TIME}"
	@echo "Git Commit: ${GIT_COMMIT}"

# Release targets
release-prep: clean test build-all ## Prepare for release
	@echo "Release preparation complete"

# Development helpers
gen-certs: ## Generate SSL certificates for testing
	mkdir -p certs
	openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/C=US/ST=CA/L=San Francisco/O=PorterFS/OU=Testing/CN=localhost"

setup: deps gen-certs ## Set up development environment
	cp config.yaml.example config.yaml
	mkdir -p data data-ssl
	@echo "Development environment set up successfully"
	@echo "Run 'make run' to start the server"

setup-test-data: ## Set up test data directories
	mkdir -p data data-ssl
	@echo "Test data directories created"