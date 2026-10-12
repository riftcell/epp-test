# How-To: Integration Testing

This guide explains the three testing tiers available in the EPP Test Framework. Each tier targets a different stage in the development workflow, from fast offline iteration to live OT&E validation.

## Overview

| Tier | Command | Network | Credentials | What it validates |
|------|---------|---------|-------------|-------------------|
| 1 — In-process unit tests | `go test -tags unit ./...` | None (offline) | None | Protocol framing, adapter logic, scenario runner, all EPP operations |
| 2 — Standalone simulated server | `make mock-epp-server` / `make mock-rri-server` or `docker run riftcell/epp-mock` | Loopback only | None | Client wire framing, TLS handshake, connect flow, error handling (via fault injection) |
| 3 — Real OT&E integration | `go test -tags integration ./...` | Internet | Required | End-to-end EPP operations against live OT&E endpoints |

---

## Tier 1: In-Process Unit Tests

In-process mock servers are created and scripted inside test code via `NewMockEPPServer(t)` and `NewMockRRIServer(t)`. They listen on OS-assigned ports (`:0`), require no setup, run offline, and clean up automatically.

```sh
go test -tags unit ./...
```

This is the default mode for CI. It covers all protocol logic and scenario steps without a network connection.

The in-process mock is scripted: the test queues the greeting with `srv.InjectPoll(...)` and one response per command on `srv.Expect`, then drives the adapter. [Getting Started](getting-started.md#your-first-test) has a complete, runnable example, and [`scenarios/conformance/conformance_test.go`](../scenarios/conformance/conformance_test.go) shows how to run full YAML scenarios this way.


---

## Tier 2: Standalone Simulated Server

A long-running mock server runs as a separate OS process. Your client connects to it over loopback — exactly like a real server, but with no credentials and no OT&E account needed.

> **Client not written in Go?** Run the servers from the `riftcell/epp-mock` Docker image instead. [How-To: Docker Mock](how-to-docker-mock.md) covers the `docker run` commands, wire framing, TLS settings and PHP examples.

### Starting the Servers

```sh
# EPP TLS mock server on 127.0.0.1:7700
go run ./cmd/mock-epp-server
# or
make mock-epp-server

# RRI TCP mock server on 127.0.0.1:7701
go run ./cmd/mock-rri-server
# or
make mock-rri-server
```

Both servers log incoming commands to stderr and shut down cleanly on Ctrl+C or SIGTERM.

Use `-addr` to change the listen address:

```sh
go run ./cmd/mock-epp-server -addr 0.0.0.0:7700
go run ./cmd/mock-rri-server -addr 0.0.0.0:7701
```

### EPP Server Behavior

- **TLS with no client-cert requirement.** The server generates a self-signed ECDSA P-256 certificate at startup. Any EPP client can connect by either trusting the printed cert or using `InsecureSkipVerify: true` for quick manual testing.
- **Greeting sent immediately on connect.** Real EPP servers always speak first.
- **Always-success responses.** Login, domain, contact, and host operations all return result code 1000. `domain:check` marks all queried names as available (`avail="1"`). Logout returns 1500 and closes the connection.
- **Logs command types to stderr.** Watch for `epp <- login`, `epp <- domain:check`, etc. (with Docker, `docker logs` shows them).

### RRI Server Behavior

- **Plain TCP, no TLS.** Matches the real DENIC RRI wire protocol.
- **No credential validation.** Any LOGIN is accepted as success.
- **Returns `RESULT: success` for every command** with an incrementing STID counter.
- **Closes after LOGOUT.** The real DENIC server closes the connection after LOGOUT; go-rriclient waits for the EOF.

### Pointing a Client at the Standalone Server

Here is a Go test snippet that connects to the running mock EPP server and exercises the session flow:

```go
package mytest_test

import (
    "context"
    "crypto/tls"
    "testing"

    "github.com/riftcell/epp-test/pkg/config"
    "github.com/riftcell/epp-test/pkg/registrar/epp"
)

func TestAgainstStandaloneEPPServer(t *testing.T) {
    cfg := config.RegistrarConfig{
        Host:     "127.0.0.1",
        Port:     7700,
        Username: "any-user",
        Password: "any-password",
    }
    reg, err := epp.NewGenericEPPAdapter("mock", cfg,
        epp.WithTLSConfig(&tls.Config{
            InsecureSkipVerify: true, //nolint:gosec // intentional for standalone mock
        }),
    )
    if err != nil {
        t.Fatalf("create adapter: %v", err)
    }
    defer reg.Close()

    ctx := context.Background()
    if err := reg.Login(ctx); err != nil {
        t.Fatalf("login: %v", err)
    }
    // The server always returns code 1000 — this validates wire framing and connect flow.
    results, err := reg.CheckDomain(ctx, "example.com")
    if err != nil {
        t.Fatalf("check domain: %v", err)
    }
    for _, r := range results {
        t.Logf("%s available: %v", r.Name, r.Available != nil && *r.Available)
    }
}
```

Note that the standalone server always returns 1000 success; it validates that the client can frame requests and parse responses, not that registrar-specific logic is correct. Use Tier 3 for that.

---

## Tier 3: Real OT&E Integration

Run the full conformance suite against live OT&E sandbox endpoints:

```sh
go test -tags integration ./...
```

This requires registrar credentials. See [Config Reference](config-reference.md) for all `RegistrarConfig` fields and their environment variable overrides.

Quick example for a single registrar:

```sh
EPP_REGISTRARS_INTERNETX_PASSWORD=secret \
EPP_REGISTRARS_INTERNETX_USERNAME=myuser \
go test -tags integration -run TestInternetX -timeout 30m -v ./...
```

OT&E accounts for InternetX, NiCAT, EURid, and DENIC must be requested from each registrar before running Tier 3 tests.

---

## Docker Compose: Mock Server + Client Tests

The following illustrative `docker-compose.yaml` runs the standalone mock servers from the Dockerfile's `mock-servers` stage (static binaries on `scratch`; no Go toolchain in the image; published as `riftcell/epp-mock`) next to your client under test. The servers bind `0.0.0.0` inside the container so other services can reach them.

```yaml
services:
  mock-epp:
    build: { context: ., target: mock-servers }
    # -san adds the service name to the self-signed cert, so clients dialing
    # "mock-epp" pass hostname verification (default SANs: localhost, 127.0.0.1).
    command: ["-addr", "0.0.0.0:7700", "-san", "mock-epp"]
    ports: ["7700:7700"]

  mock-rri:
    build: { context: ., target: mock-servers }
    entrypoint: ["/mock-rri-server"]
    command: ["-addr", "0.0.0.0:7701"]
    ports: ["7701:7701"]

  client-tests:
    image: your-client-under-test   # replace with your EPP client's test image
    depends_on: [mock-epp, mock-rri]
    # Point your client at mock-epp:7700 (EPP over TLS) and mock-rri:7701 (RRI).
```

The mock server's certificate is regenerated on every start and is self-signed, so the client must either skip verification or trust the cert obtained from the first handshake. Fault-injection flags (see [Fault Injection](#fault-injection)) can be appended to `command`, e.g. `["-addr", "0.0.0.0:7700", "-san", "mock-epp", "--login-mode=flap"]`.

See [How-To: Docker Mock](how-to-docker-mock.md) for the wire protocol details and example client code.

This setup is useful for validating wire framing in a containerized CI environment without an OT&E account.

---

## Quick Reference

| Scenario | Use tier |
|----------|----------|
| Fast offline CI, no setup | Tier 1 — `go test -tags unit ./...` |
| Manual client debugging, no OT&E account | Tier 2 — `make mock-epp-server` |
| Final OT&E validation before release | Tier 3 — `go test -tags integration ./...` |
| Docker CI pipeline without OT&E | Tier 2 in compose |

---

## Fault Injection

Both standalone mock servers support configurable fault simulation to verify that your EPP
or RRI client library handles failure conditions correctly — without touching a live OT&E
endpoint.

The five fault categories are:

- **Connection delay** — sleep before TLS greeting / first RRI read (`--connect-delay`)
- **Login failures** — always fail, flap (fail-then-succeed), hang, disconnect (`--login-mode`)
- **Abrupt disconnection** — pre-greeting, post-greeting, after N ops, or on a specific op
- **Malformed EPP frames** — length overflow, invalid XML, raw garbage (`--malformed-frame`)
- **Logical mismatch** — success response naming the wrong resource (`--fault-mismatch`)

Quick example — test login failure handling:

```sh
go run ./cmd/mock-epp-server --login-mode=always
```

See [how-to-fault-injection.md](how-to-fault-injection.md) for the complete reference:
all CLI flags, YAML profile format, per-operation rules, and what to assert in client tests.
