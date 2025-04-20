# Go parameters
BINARY_NAME=tix
MAIN_PACKAGE=.
GOBIN=$(CURDIR)/bin

# Version parameters
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LD_FLAGS=-ldflags "-X github.com/tedkulp/tix/internal/version.Version=$(VERSION) -X github.com/tedkulp/tix/internal/version.Commit=$(COMMIT) -X github.com/tedkulp/tix/internal/version.Date=$(BUILD_DATE)"

# Tools
GOLANGCI_LINT = $(shell which golangci-lint 2>/dev/null)

.PHONY: all build clean test test-verbose run install lint vet format help install-tools

help:
	@echo "Available commands:"
	@echo "  make              - Build and test the application"
	@echo "  make build        - Build the application"
	@echo "  make clean        - Remove built binary and bin directory"
	@echo "  make test         - Run tests"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make test-coverage - Run tests and generate coverage report"
	@echo "  make run          - Build and run the application"
	@echo "  make install      - Install the application"
	@echo "  make lint         - Run linting tools"
	@echo "  make vet          - Run go vet"
	@echo "  make format       - Format code using gofmt"
	@echo "  make install-tools - Install required tools" 

all: clean build test

build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(GOBIN)
	@go build $(LD_FLAGS) -o $(GOBIN)/$(BINARY_NAME) $(MAIN_PACKAGE)

clean:
	@echo "Cleaning..."
	@rm -rf $(GOBIN)

test:
	@echo "Running tests..."
	@go test ./...

test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

run: build
	@echo "Running $(BINARY_NAME)..."
	@$(GOBIN)/$(BINARY_NAME)

install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LD_FLAGS) $(MAIN_PACKAGE)

lint: install-tools
	@echo "Linting code..."
	@golangci-lint run ./...

vet:
	@echo "Vetting code..."
	@go vet ./...

format:
	@echo "Formatting code..."
	@gofmt -w .

install-tools:
ifeq ($(GOLANGCI_LINT),)
	@echo "Installing golangci-lint..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
endif