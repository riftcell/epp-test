.PHONY: all test test-unit test-integration lint lint-fix build tidy clean docker-build docker-run docker-test docker-selftest mock-epp-server mock-rri-server

all: lint test-unit build

# Run untagged tests — validates the module compiles cleanly.
# No untagged test files exist yet; this target should exit 0 with "no test files".
test:
	go test ./...

# Run unit tests (mock-server-backed, offline).
# -race: detect data races early (mock servers use goroutines)
# -count=1: disable test result cache
test-unit:
	go test -tags unit -race -count=1 ./...

# Run integration tests for one registrar.
# Usage: make test-integration REGISTRAR=internetx
test-integration:
	go test -tags integration -run 'Test$(REGISTRAR)' -timeout 30m -v ./...

# Lint all packages. Requires golangci-lint in PATH.
# Install: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

# Build all packages as a static binary (verifies CGO_ENABLED=0 constraint, INFRA-03).
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" ./...

# Tidy go.mod and go.sum.
tidy:
	go mod tidy

# Build the Docker image (multi-stage: golang:1.25-alpine builder → scratch runtime).
# Produces a static conformance test binary with zero runtime dependencies (CICD-01).
docker-build:
	docker build -t epp-test-framework .

# Run the conformance suite inside the Docker image.
# Expects no Go toolchain on the host — the binary is self-contained.
docker-run:
	docker run --rm epp-test-framework

# Build and immediately run (convenience target for CI and local verification).
docker-test: docker-build docker-run

# Build and run the full unit suite (all packages) inside Docker.
# Uses the `selftest` stage which keeps the Go toolchain — image is larger (~300 MB)
# but runs `go test -tags unit ./...` across every package without needing Go on the host.
docker-selftest:
	docker build --target selftest -t epp-test-framework-selftest .
	docker run --rm epp-test-framework-selftest

# Run the standalone mock EPP server (TLS, 127.0.0.1:7700) for manual integration testing.
# Use -addr to change the listen address: make mock-epp-server ARGS="-addr 0.0.0.0:7700"
mock-epp-server:
	go run ./cmd/mock-epp-server $(ARGS)

# Run the standalone mock RRI server (TCP, 127.0.0.1:7701) for manual integration testing.
# Use -addr to change the listen address: make mock-rri-server ARGS="-addr 0.0.0.0:7701"
mock-rri-server:
	go run ./cmd/mock-rri-server $(ARGS)

# Remove generated binaries.
clean:
	rm -rf bin/
