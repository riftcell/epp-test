# Architecture

## 4-Layer Design

The framework is organized into four horizontal layers. Each layer depends only on the layer below it; upper layers are swappable without touching lower ones.

```text
+----------------------------------------------------------+
|                     YAML Scenarios                       |
|  scenarios/conformance/*.yaml                            |
|  domain_lifecycle, contact_lifecycle, host_lifecycle,    |
|  poll_lifecycle, denic_rri, negative_tests, ...          |
+----------------------------------------------------------+
              |  pkg/runner.RunScenario / RunMatrix / RunDir
              v
+----------------------------------------------------------+
|                  Registrar Interface                     |
|  pkg/registrar.Registrar                                 |
|  DomainManager | ContactManager | HostManager | Poller   |
|  Login | Logout | Ping | Name                            |
+----------------------------------------------------------+
              |  implemented by
              v
+----------------------------------------------------------+
|                      Adapters                            |
|  pkg/registrar/epp/  GenericEPPAdapter                   |
|    - InternetXAdapter (embed, nil hooks)                 |
|    - NiCATAdapter     (embed, OnBuildContactCreate hook) |
|    - EURidAdapter     (embed, 3 extension hooks)         |
|  pkg/registrar/rri/  RRIAdapter                          |
|    - DENICAdapter     (embed RRIAdapter)                 |
+----------------------------------------------------------+
              |  dial + send frames
              v
+----------------------------------------------------------+
|               Servers (mock or real)                     |
|  pkg/mock/epp.MockEPPServer  — unit tests (in-process)   |
|  pkg/mock/rri.MockRRIServer  — unit tests (in-process)   |
|  OT&E / sandbox endpoints    — integration tests         |
+----------------------------------------------------------+
```

## Layer Descriptions

**YAML Scenarios** are declarative test programs. Each `.yaml` file in `scenarios/conformance/` defines a sequence of steps (operations and expectations), a matrix of registrars to run against, and optional per-registrar parameter overrides. Writing a test means writing YAML — no Go code unless you need a custom assertion.

**Registrar Interface** (`pkg/registrar`) is the shared contract. It is the only thing the scenario runner (`pkg/runner`) knows about. Every method carries `context.Context` as the first argument for timeout and cancellation. The interface is split into composable sub-interfaces (`DomainManager`, `ContactManager`, `HostManager`, `Poller`) so callers can depend on just the subset they need.

**Adapters** translate `Registrar` method calls into wire-format commands. `GenericEPPAdapter` handles RFC 5730–5734 XML framing, TLS connection management, reconnection, and login/logout. Per-registrar adapters (`InternetXAdapter`, `NiCATAdapter`, `EURidAdapter`) embed `GenericEPPAdapter` and wire in extension-specific hooks without re-implementing core EPP logic. `RRIAdapter` handles DENIC's text-based TCP protocol (not XML); `DENICAdapter` embeds it. All adapters satisfy the same `Registrar` interface — the runner treats them identically.

**Servers** are where commands land. In unit tests the adapters dial in-process mock servers (`pkg/mock/epp`, `pkg/mock/rri`) that listen on `127.0.0.1:0` (OS-assigned port) and replay scripted responses from a channel. In integration tests the adapters connect to real OT&E endpoints. No test code changes — only the `RegistrarConfig` (host/port/credentials) changes.

## How to add a new registrar

Adding support for a new EPP-based registrar takes four steps.

**Step 1: Create an adapter type that embeds `GenericEPPAdapter`.**

```go
// pkg/registrar/epp/acme.go

package epp

// AcmeAdapter implements the Registrar interface for the Acme EPP registry.
// It embeds GenericEPPAdapter; all standard EPP operations are inherited.
type AcmeAdapter struct {
    *GenericEPPAdapter
}

// Compile-time assertion: AcmeAdapter must satisfy registrar.Registrar.
var _ registrar.Registrar = (*AcmeAdapter)(nil)
```

**Step 2: Write a constructor that configures extension hooks.**

```go
func NewAcmeAdapter(cfg config.RegistrarConfig, opts ...Option) (*AcmeAdapter, error) {
    base, err := NewGenericEPPAdapter(cfg, opts...)
    if err != nil {
        return nil, err
    }
    a := &AcmeAdapter{GenericEPPAdapter: base}

    // Wire Acme-specific extension hook (if any).
    // Leave hooks nil if the registrar uses no proprietary extensions.
    base.OnBuildDomainCreate = a.buildDomainCreateExt
    return a, nil
}
```

**Step 3: Return the registrar's canonical name.**

```go
func (a *AcmeAdapter) Name() string { return "acme" }
```

**Step 4: Add the new registrar key to `config/config.go` explicit `BindEnv` calls.**

In `Load()`, add `"acme"` to the `[]string{"internetx", "nicat", "eurid", "denic"}` slice so environment variable overrides (`EPP_REGISTRARS_ACME_PASSWORD`) work in CI.

After these four steps, add `acme` to your `epp-test.yaml`, run `go test -tags unit ./...` to confirm the compile-time assertion passes, then write a scenario with `matrix: [acme]` and run it against the OT&E endpoint.

**Templates to copy from:** `pkg/registrar/epp/internetx.go` (no hooks, thinnest possible adapter), `pkg/registrar/epp/nicat.go` (single hook), `pkg/registrar/epp/eurid.go` (three hooks covering five extension namespaces).

## Key Design Decisions

- **`github.com/nbio/xml` instead of `encoding/xml`** — Go's built-in XML marshaler drops EPP namespace prefixes (`domain:create` becomes `ns0:create`), which registrars reject. `nbio/xml` is a drop-in replacement that preserves prefixes. See [Go issue #48821](https://github.com/golang/go/issues/48821).
- **EPP frame length includes the 4-byte header** — RFC 5734 specifies that the length field counts itself. A 100-byte payload requires a header value of 104. This off-by-four error is the most common EPP implementation bug; it is unit-tested in `pkg/epp/frame/`.
- **DENIC RRI is text-based, not XML** — DENIC's RRI protocol uses a `key:value` line format over plain TCP with MD5-hashed passwords. `RRIAdapter` wraps `github.com/DENICeG/go-rriclient` and bridges it to the `Registrar` interface.
- **Build tags separate test layers** — `//go:build unit` for mock-server tests, `//go:build integration` for OT&E tests. `go test ./...` runs only compile-time assertions. See [CONVENTIONS.md](../CONVENTIONS.md) section 8.
