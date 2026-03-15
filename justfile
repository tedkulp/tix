binary_name := "tix"
main_package := "."
gobin := justfile_directory() / "bin"

version := `git describe --tags --always --dirty 2>/dev/null || echo "0.1.0"`
commit := `git rev-parse --short HEAD 2>/dev/null || echo "dev"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ld_flags := "-ldflags \"-X github.com/tedkulp/tix/internal/version.Version=" + version + " -X github.com/tedkulp/tix/internal/version.Commit=" + commit + " -X github.com/tedkulp/tix/internal/version.Date=" + build_date + "\""

# Build and test the application
default: clean build test

# Build the application
build:
    @echo "Building {{binary_name}} {{version}}..."
    @mkdir -p {{gobin}}
    go build {{ld_flags}} -o {{gobin}}/{{binary_name}} {{main_package}}

# Remove built binary and bin directory
clean:
    @echo "Cleaning..."
    @rm -rf {{gobin}}

# Run tests
test:
    @echo "Running tests..."
    go test ./...

# Run tests with verbose output
test-verbose:
    @echo "Running tests with verbose output..."
    go test -v ./...

# Run tests and generate coverage report
test-coverage:
    @echo "Running tests with coverage..."
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out

# Build and run the application
run: build
    @echo "Running {{binary_name}}..."
    {{gobin}}/{{binary_name}}

# Install the application
install: build
    @echo "Installing {{binary_name}}..."
    go install {{ld_flags}} {{main_package}}

# Run linting tools
lint: install-tools
    @echo "Linting code..."
    golangci-lint run ./...

# Run linting tools and automatically fix issues
lint-fix: install-tools
    @echo "Linting and fixing code..."
    golangci-lint run --fix ./...

# Run go vet
vet:
    @echo "Vetting code..."
    go vet ./...

# Format code using gofmt
format:
    @echo "Formatting code..."
    gofmt -w .

# Install required tools
install-tools:
    #!/usr/bin/env sh
    if ! which golangci-lint > /dev/null 2>&1; then
        echo "Installing golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    fi
