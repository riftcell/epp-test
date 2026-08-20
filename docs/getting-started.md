# Getting Started

This guide walks you from a clean machine to a passing `go test -tags unit ./...` in under 10 minutes. No real registrar credentials are needed for unit tests — they run entirely against in-process mock servers.

## Prerequisites

- **Go 1.25 or later.** Check with `go version`. The framework uses generics and standard library features added in 1.20+; 1.25 is the minimum declared in `go.mod`.
- No other system dependencies. The framework compiles as a fully static binary (`CGO_ENABLED=0`). No C libraries, no database, no external runtime.

## Install / Module Setup

**Option A: Clone the repository**

```sh
git clone https://github.com/riftcell/epp-test
cd epp-test
```

**Option B: Add as a dependency to your own project**

```sh
go get github.com/riftcell/epp-test
```

## Config File

Unit tests use in-process mock servers and do not connect to any external endpoint, so credentials are ignored at the unit level. However, the `config.Load()` function validates that required fields are present when a YAML file is found. Copy the example config to avoid validation errors:

```sh
cp configs/epp-test.example.yaml epp-test.yaml
```

The example config (`configs/epp-test.example.yaml`) looks like this:

```yaml
# epp-test.yaml — EPP Test Framework Configuration
#
# Copy this file to epp-test.yaml and fill in your OT&E credentials.
# Never commit epp-test.yaml with real credentials — use environment variable
# overrides for CI:
#
#   EPP_REGISTRARS_INTERNETX_PASSWORD=secret go test -tags integration -run TestInternetX ./...
#
# Environment variable format:
#   EPP_REGISTRARS_<NAME>_<FIELD>
#   where <NAME> and <FIELD> are uppercase with underscores replacing dots.

registrars:
  internetx:
    host: epp.internetx.de
    port: 700
    username: test-user
    password: secret
    cert_file: /certs/internetx/client.pem
    key_file: /certs/internetx/client.key
    ca_file: /certs/internetx/ca.pem
    extensions:
      - urn:ietf:params:xml:ns:domain-1.0
      - urn:ietf:params:xml:ns:contact-1.0
      - urn:ietf:params:xml:ns:host-1.0

  nicat:
    host: epp.nic.at
    port: 700
    username: nic-user
    password: nic-secret
    cert_file: /certs/nicat/client.pem
    key_file: /certs/nicat/client.key
    extensions:
      - http://www.nic.at/xsd/at-ext-verification-1.0

  eurid:
    host: epp.eurid.eu
    port: 700
    username: eurid-user
    password: eurid-secret
    extensions:
      - http://www.eurid.eu/xml/epp/contact-ext-1.3
      - http://www.eurid.eu/xml/epp/domain-ext-2.4

  denic:
    host: epp.denic.de
    port: 700
    username: denic-user
    password: denic-secret
    extensions: []
```

Config file discovery order (first match wins):

1. `$EPP_CONFIG_FILE` — environment variable pointing to an absolute path
2. `./epp-test.yaml` — current working directory
3. `$HOME/.epp-test/epp-test.yaml` — user home directory

For the unit suite, credentials in the YAML file are not validated against any live server, so you can leave the example values as-is.

## Your First Test

Create a file `mytest_test.go` with the following content:

```go
//go:build unit

package mytest_test

import (
    "testing"

    "github.com/riftcell/epp-test/pkg/mock/epp"
    "github.com/riftcell/epp-test/pkg/runner"
)

func TestDomainLifecycle(t *testing.T) {
    // Start an in-process mock EPP server on a random port.
    // t.Cleanup automatically closes the server when the test ends.
    srv := epp.NewMockEPPServer(t)

    // Create an adapter pointing at the mock server.
    reg := srv.NewAdapter(t)

    // Run the standard domain lifecycle conformance scenario.
    runner.RunScenario(t, "scenarios/conformance/domain_lifecycle.yaml", reg)
}
```

Then run the unit suite:

```sh
go test -tags unit ./...
```

Expected output (trimmed):

```
ok      github.com/riftcell/epp-test/...   0.123s
```

All tests pass. The scenario exercises `CheckDomain`, `CreateDomain`, `InfoDomain`, `UpdateDomain`, `RenewDomain`, `TransferDomain`, and `DeleteDomain` against the mock server without any network calls.

## Build Tags

The framework uses Go build tags to separate test layers:

| Command | What runs |
|---------|-----------|
| `go test ./...` | Compile-time assertions only (fastest, always offline) |
| `go test -tags unit ./...` | Full unit suite using in-process mock servers |
| `go test -tags integration ./...` | Integration suite against real OT&E endpoints |

See [CONVENTIONS.md](../CONVENTIONS.md) section 8 for the full build tag policy.

## Next Steps

- [How-To: Domains](how-to-domains.md) — domain lifecycle operations in detail
- [How-To: Contacts](how-to-contacts.md) — contact create, info, update, delete
- [How-To: Hosts](how-to-hosts.md) — host (nameserver) lifecycle
- [How-To: Polling](how-to-polling.md) — EPP message queue operations
- [How-To: DENIC RRI](how-to-denic-rri.md) — DENIC-specific RRI protocol
- [Config Reference](config-reference.md) — every config field documented
- [Architecture](architecture.md) — layered design and how to add a new registrar
