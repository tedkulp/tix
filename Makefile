# Go parameters
BINARY_NAME=tix
MAIN_PACKAGE=.
GOBIN=$(CURDIR)/bin

.PHONY: all build clean test test-verbose run install lint vet format help

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

all: clean build test

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(GOBIN)
	@go build -o $(GOBIN)/$(BINARY_NAME) $(MAIN_PACKAGE)

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

install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(MAIN_PACKAGE)

lint:
	@echo "Linting code..."
	@test -z $(shell gofmt -l .)
	@go vet ./...

vet:
	@echo "Vetting code..."
	@go vet ./...

format:
	@echo "Formatting code..."
	@gofmt -w .