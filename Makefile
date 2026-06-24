.PHONY: build test test-cover lint run clean deps

# Build the binary.
build:
	go build -o bin/bridge ./cmd/bridge/

# Run all tests with race detector.
test:
	go test -race -count=1 ./...

# Run tests with coverage report.
test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run linter (requires golangci-lint).
lint:
	golangci-lint run ./...

# Build and run with example config.
run: build
	./bin/bridge -config config.example.yaml

# Run with debug logging.
run-debug: build
	./bin/bridge -config config.example.yaml -debug

# Download dependencies.
deps:
	go mod download
	go mod tidy

# Clean build artifacts.
clean:
	rm -rf bin/ coverage.out coverage.html
