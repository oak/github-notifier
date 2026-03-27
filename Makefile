.PHONY: test test-verbose test-coverage test-race lint build clean help register-macos-notifier

# Default target
.DEFAULT_GOAL := help

## test: Run all tests
test:
	@echo "Running tests..."
	@go test ./...

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=coverage.out | grep total:
	@echo ""
	@echo "=== Coverage by Package ==="
	@go tool cover -func=coverage.out | grep -E '(domain|application|infrastructure)' | head -20 || true
	@echo ""
	@echo "To view detailed HTML coverage report, run: make coverage-html"

## coverage-html: Generate and open HTML coverage report
coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

## test-integration: Run only integration tests (if any)
test-integration:
	@echo "Running integration tests..."
	@go test -v -tags=integration ./test/integration/... || echo "No integration tests found"

## test-e2e: Run end-to-end tests
test-e2e:
	@echo "Running E2E tests..."
	@go test -v -tags=e2e ./test/e2e/...

## test-all: Run all test types (unit, integration, e2e)
test-all: test test-e2e
	@echo "All tests completed!"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w -local github.com/oak3/github-notifier .

## build: Build the application
build:
	@echo "Building github-notifier..."
	@go build -o github-notifier ./cmd/github-notifier

## clean: Remove build artifacts and coverage files
clean:
	@echo "Cleaning..."
	@rm -f github-notifier coverage.out coverage.html
	@go clean

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/vektra/mockery/v2@latest

## mocks: Generate mocks
mocks:
	@echo "Generating mocks..."
	@go run github.com/vektra/mockery/v2@latest --name=PullRequestRepository --dir=domain/pullrequest --output=internal/mocks --outpkg=mocks
	@go run github.com/vektra/mockery/v2@latest --name=EventPublisher --dir=application/port --output=internal/mocks --outpkg=mocks
	@go run github.com/vektra/mockery/v2@latest --name=EventHandler --dir=application/port --output=internal/mocks --outpkg=mocks
	@go run github.com/vektra/mockery/v2@latest --name=NotificationPort --dir=application/port --output=internal/mocks --outpkg=mocks
	@go run github.com/vektra/mockery/v2@latest --name=UIPort --dir=application/port --output=internal/mocks --outpkg=mocks
	@go run github.com/vektra/mockery/v2@latest --name=Service --dir=domain/tracking --output=internal/mocks --outpkg=mocks
	@echo "Mocks generated successfully!"

## register-macos-notifier: Register a local app bundle as the macOS notification sender (macOS only)
register-macos-notifier:
	@chmod +x scripts/register-macos-notifier.sh
	@./scripts/register-macos-notifier.sh

## ci: Run all CI checks (test, lint, build)
ci: test-race lint build
	@echo "All CI checks passed!"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
