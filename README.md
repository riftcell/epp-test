# EPP Test Framework

A Go test framework for EPP (Extensible Provisioning Protocol) client libraries, with first-class support for InternetX, NiCAT, EURid, and DENIC. Covers the full EPP operation surface — domains, contacts, hosts, polling — plus DENIC's proprietary text-based RRI protocol. Tests run locally against in-process mock servers and against OT&E/sandbox environments using the same test code and without any changes to configuration.

**Core value:** Give EPP client developers fast, reliable, locally-runnable tests that also work against real registrar environments. Write a scenario once; run it offline against a mock server or live against an OT&E endpoint by switching a config file.

## Quick Start

```sh
# 1. Clone the repository (or add as a Go module dependency)
git clone https://github.com/riftcell/epp-test && cd epp-test

# 2. Copy the example config (unit tests need no real credentials)
cp configs/epp-test.example.yaml epp-test.yaml

# 3. Run the unit test suite
go test -tags unit ./...
```

## Documentation

| Guide | Description |
| ----- | ----------- |
| [Getting Started](docs/getting-started.md) | Install, configure, and run your first test in under 10 minutes |
| [How-To: Domains](docs/how-to-domains.md) | Domain lifecycle: check, create, info, update, renew, transfer, delete |
| [How-To: Contacts](docs/how-to-contacts.md) | Contact lifecycle: create, info, update, delete, domain association |
| [How-To: Hosts](docs/how-to-hosts.md) | Host lifecycle: create subordinate nameserver, info, update glue, delete |
| [How-To: Polling](docs/how-to-polling.md) | EPP message queue: trigger, wait, assert content, ack, verify empty |
| [How-To: DENIC RRI](docs/how-to-denic-rri.md) | DENIC-specific RRI operations: TRANSIT, AUTHINFO1/AUTHINFO2, info, delete |
| [Config Reference](docs/config-reference.md) | Every `RegistrarConfig` field with env var, type, default, and validation |
| [Architecture](docs/architecture.md) | 4-layer diagram and how to add a new registrar |
| [Conventions](CONVENTIONS.md) | Go coding conventions for this project |
| [Security](SECURITY.md) | Security policy and vulnerability reporting |
| [How-To: Integration Testing](docs/how-to-integration-testing.md) | Three-tier testing strategy: in-process, standalone mock server, and real OT&E |
| [License](LICENSE) | MIT license terms for this project |
| [Contributors](CONTRIBUTORS) | People who have contributed to this project |

## Test Modes

| Command | What runs |
| ------- | --------- |
| `go test ./...` | Compile-time assertions only (no network, no mock servers) |
| `go test -tags unit ./...` | Full unit suite using in-process mock servers (offline, fast) |
| `make mock-epp-server` / `make mock-rri-server` | Long-running mock server in a separate process for manual integration testing |
| `go test -tags integration ./...` | Integration suite against OT&E endpoints (requires credentials) |

See [How-To: Integration Testing](docs/how-to-integration-testing.md) for a detailed walkthrough of all three tiers, including Docker Compose examples.

## Module Path

```
github.com/riftcell/epp-test
```

Requires Go 1.25 or later.
