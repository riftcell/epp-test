<!-- GSD:project-start source:PROJECT.md -->
## Project

**EPP Test Framework**

A Go test framework for EPP (Extensible Provisioning Protocol) client libraries, with first-class support for InternetX, NiCAT, EURid, and DENIC. Covers the full EPP operation surface (domains, contacts, hosts, polling) plus DENIC's proprietary text-based RRI protocol. Tests run locally against mock servers and against OT&E/sandbox environments, and are portable to any CI/CD system via Docker/shell.

**Core Value:** Give EPP client developers fast, reliable, locally-runnable tests that also work against real registrar environments without changing the test code.

### Constraints

- **Language**: Go — must integrate with standard `go test ./...` toolchain
- **Test isolation**: Unit tests must run offline with no external network access
- **Config**: YAML/TOML config files; no hardcoded credentials anywhere in test code
- **Protocol**: DENIC RRI is text-based TCP (not XML); mock and client must implement this faithfully
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### Core EPP / Protocol
#### EPP XML: Roll Your Own Structs with `encoding/xml` + `github.com/nbio/xml`
| Library | State | Why Not |
|---|---|---|
| `github.com/domainr/epp` v0.2.0 | Unstable, API breaks any time. Only `Check` implemented, no tests in lib. | Incomplete — no Create/Delete/Update/Info/Poll |
| `github.com/domainr/epp2` | Experimental next-version fork, no releases, directs users back to epp | Premature |
| `github.com/0x4b53/epp-go` | "Work in progress, a long way from completed." Inactive. | Dead |
| `github.com/pixel365/goepp` | RFC 5730-5734 parser, work in progress, 0 stars/forks | Too new, no adoption |
| `github.com/BoltNGroup/go-epp` | Clone of domainr/epp | Same limitations |
| `github.com/onasunnymorning/eppclient` | Active (2021-2025), partial | Unverified scope |
#### DENIC RRI: `github.com/DENICeG/go-rriclient/pkg/rri` v1.26.0
#### TLS Client Certificates: `crypto/tls` (stdlib)
### Test Infrastructure
#### Assertions: `github.com/stretchr/testify` v1.11.1
- `testify/assert` — non-fatal assertions
- `testify/require` — fatal assertions (stops test on failure, important for stateful scenarios)
- `testify/suite` — per-registrar test suites with `SetupSuite`/`TearDownSuite` lifecycle hooks
#### Mock Generation: `github.com/vektra/mockery/v2` v2.x
### Mock Server Pattern
#### Pattern: `net.Listen("tcp", "127.0.0.1:0")` with goroutine accept loop
- Listen on `:0` (port 0) to get an OS-assigned port — avoids port conflicts in CI
- Use `t.Cleanup(server.Close)` for automatic teardown — no manual `defer` needed
- The `handler` func receives a raw `net.Conn`, reads EPP frames, and writes scripted responses
- For scenario-driven tests, the handler reads the next expected response from a channel fed by the test
### YAML Scenario Runner
#### Pattern: Custom runner over `gopkg.in/yaml.v3`
- `testy` (YAML HTTP tests) is HTTP-specific and cannot drive a custom `RegistrarClient` interface
- `ditrit/specimen` is early-stage with no meaningful adoption
- All existing YAML test runners are built for HTTP — none understand EPP session state
### CI/CD / Docker
#### Pattern: Multi-stage Dockerfile + shell entrypoints + build tag env vars
## What NOT to Use
| Rejected Option | Reason |
|---|---|
| `github.com/domainr/epp` or any existing Go EPP client as a hard dependency | None have full RFC coverage (Create/Delete/Update/Info/Poll). Unstable APIs. For a test framework that must validate an EPP client library, owning the wire-format structs is safer than depending on another incomplete client. |
| `github.com/0x4b53/epp-go` | Inactive, self-described as incomplete, recommends a different library |
| `golang/mock` (archived) or `uber-go/mock` | `golang/mock` is archived. `uber-go/mock` is a fork with less momentum than mockery. mockery + testify/mock is the current standard. |
| `sigs.k8s.io/yaml` for scenario YAML | YAML-to-JSON conversion loses anchors and sequence types. Only appropriate for Kubernetes-ecosystem projects that need JSON tag compatibility. |
| `testy` or other YAML HTTP test runners | All existing YAML test frameworks target HTTP. They cannot drive a `net.Conn`-based EPP client interface. |
| `encoding/xml` stdlib alone (without nbio/xml) | Cannot control namespace prefixes on marshal. EPP registrars validate namespace declarations strictly. nbio/xml is a minimal drop-in that fixes this. |
| Viper for scenario files | Viper is for config (credentials, endpoints). Scenario YAML has a domain-specific structure with steps/expectations that maps better to direct yaml.v3 unmarshaling into typed structs. |
| Any test framework that requires a running HTTP server (e.g. gRPC test frameworks) | EPP is raw TCP + TLS, not HTTP. HTTP-oriented test tooling does not apply. |
## Confidence Levels
| Area | Level | Basis |
|---|---|---|
| TLS client certificate pattern (`crypto/tls`) | HIGH | Stdlib, official docs, no alternatives |
| EPP 4-byte frame framing (`encoding/binary`) | HIGH | Directly in RFC 5734, stdlib |
| `nbio/xml` for namespace prefix fidelity | MEDIUM | Used by domainr/epp (same maintainer); upstream Go issue open since 2021 with no fix; single point of risk |
| Roll-your-own EPP XML structs | MEDIUM-HIGH | Only viable approach given library ecosystem state; EPP XML schema is stable per RFCs |
| `github.com/DENICeG/go-rriclient` for RRI | HIGH | Official DENIC source, v1.26.0, MIT, recently maintained |
| `testify` v1.11.1 | HIGH | Industry standard, actively maintained |
| `testify/suite` for registrar test structure | HIGH | Direct fit for stateful setup/teardown per registrar |
| `mockery/v2` for interface mocks | MEDIUM-HIGH | Current standard; v3 exists but v2 explicitly supported to 2029 |
| `net.Listen + goroutine accept loop` for mock server | HIGH | Standard Go stdlib pattern, no library risk |
| `gopkg.in/yaml.v3` for scenario files | HIGH | Stable, no Kubernetes dependency chain |
| `spf13/viper` v2 for credential config | HIGH | Industry standard, actively maintained |
| Custom YAML scenario runner | MEDIUM | Pattern is clear, but bespoke implementation with no prior art for EPP-specific stateful scenarios |
| Docker multi-stage + build tags | HIGH | Settled Go ecosystem pattern |
## Open Questions / Gaps
## Sources
- [domainr/epp on GitHub](https://github.com/domainr/epp)
- [domainr/epp2 on GitHub](https://github.com/domainr/epp2)
- [0x4b53/epp-go on GitHub](https://github.com/0x4b53/epp-go)
- [pixel365/goepp on GitHub](https://github.com/pixel365/goepp)
- [DENICeG/go-rriclient on pkg.go.dev](https://pkg.go.dev/github.com/DENICeG/go-rriclient/pkg/rri)
- [nbio/xml on GitHub](https://github.com/nbio/xml)
- [Go issue #48821: encoding/xml namespace prefix](https://github.com/golang/go/issues/48821)
- [stretchr/testify suite package](https://pkg.go.dev/github.com/stretchr/testify/suite)
- [vektra/mockery on GitHub](https://github.com/vektra/mockery)
- [golang.org/x/net/nettest](https://pkg.go.dev/golang.org/x/net/nettest)
- [crypto/tls on pkg.go.dev](https://pkg.go.dev/crypto/tls)
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)
- [spf13/viper on GitHub](https://github.com/spf13/viper)
- [Go TableDrivenTests wiki](https://go.dev/wiki/TableDrivenTests)
- [Go build tags for test separation](https://mickey.dev/posts/go-build-tags-testing/)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
