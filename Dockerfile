# Stage 1: builder — compile a static conformance test binary (CGO_ENABLED=0)
# Uses golang:1.25-alpine to match go.mod's declared go 1.25 minimum.
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy dependency manifests first for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source tree.
COPY . .

# Compile the conformance test binary statically (no libc, no CGO).
# -tags unit: only the offline/mock test files are included.
# -c:         produce a test binary instead of running tests immediately.
# -o:         write the binary to a fixed path for the final stage to copy.
RUN CGO_ENABLED=0 GOOS=linux go test -c -tags unit \
    -o /conformance.test \
    ./scenarios/conformance/

# Also compile the standalone mock servers as static binaries for the
# mock-servers stage below (used by the Docker Compose integration example).
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" \
    -o /mock-epp-server ./cmd/mock-epp-server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" \
    -o /mock-rri-server ./cmd/mock-rri-server

# Stage 2: selftest — run the full unit suite (all packages) inside the container.
# Keeps the Go toolchain so `go test ./...` can run without a host Go installation.
# Used by `make docker-selftest`; not the production conformance image.
FROM golang:1.25-alpine AS selftest

WORKDIR /build
# Separate COPY so `go mod download` layer is cached when only source changes.
COPY go.mod go.sum ./
RUN go mod download
COPY . .

CMD ["go", "test", "-tags", "unit", "-count=1", "-v", "./..."]

# Stage 3: mock-servers — standalone mock EPP (TLS :7700) and RRI (:7701) servers.
# Build with `--target mock-servers`. Defaults to the EPP server; override the
# entrypoint with /mock-rri-server for RRI. Binds 0.0.0.0 so other containers can reach it.
FROM scratch AS mock-servers

COPY --from=builder /mock-epp-server /mock-epp-server
COPY --from=builder /mock-rri-server /mock-rri-server

EXPOSE 7700 7701
ENTRYPOINT ["/mock-epp-server"]
CMD ["-addr", "0.0.0.0:7700"]

# Stage 4: runtime (default target, must stay last) — scratch means zero OS overhead, truly static binary.
# No shell, no libc, no Go toolchain at runtime (CICD-01 / INFRA-03).
FROM scratch

COPY --from=builder /conformance.test /conformance.test

# Copy the YAML scenario files so the test binary can read them at runtime.
# The runner uses relative paths (e.g., "domain_lifecycle.yaml"), so the
# working directory must match the directory containing the YAML files.
COPY --from=builder /build/scenarios/conformance/*.yaml /scenarios/conformance/

# Set the working directory to the YAML scenario directory so os.ReadFile
# resolves relative scenario paths ("domain_lifecycle.yaml" → found).
WORKDIR /scenarios/conformance

# Run the test binary with verbose output by default.
ENTRYPOINT ["/conformance.test", "-test.v"]
