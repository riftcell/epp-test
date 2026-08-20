# Go Conventions: EPP Test Framework

This document is the authoritative style guide for all packages in `github.com/riftcell/epp-test`.
Every phase executor MUST read this before writing code. These conventions exist to ensure the
framework compiles as a static binary, integrates with `go test`, and remains testable without
global state.

---

## 1. Naming

### General rules

- **Unexported by default.** Export only symbols that form a public contract. If in doubt, keep it unexported and export when genuinely needed.
- **Descriptive over abbreviated.** `RegistrarConfig` not `RegCfg`. `DomainCreateRequest` not `DCR`. `ParseDomainInfo` not `pdi`. The Go compiler costs nothing; readability does.
- **Acronyms stay uppercase.** `EPPError`, not `EppError`. `XMLNamespace`, not `XmlNamespace`. Standard exceptions: `id` (not `ID`) when used as a struct tag or map key.

### Interface naming

- Single-method interfaces end in `-er`: `DomainChecker`, `Poller`, `DomainReader`.
- Role-describing interfaces use noun form: `DomainManager`, `Registrar`, `ContactManager`.
- Do not name an interface `IFoo` or `FooInterface` — Go does not use Java-style `I` prefixes.

### Error type naming

- Error types end in `Error`, not `Err`: `EPPError`, `ValidationError`, `FrameError`.
- Sentinel error values (if any) are named `ErrFoo`: `ErrConnectionClosed`, `ErrTimeout`.
- Never name a type `FooErr` — violates `errname` linter rule enabled in this project.

### Test helpers

- Test helper functions accept `*testing.T` as first argument.
- Call `t.Helper()` as the first statement in every test helper function — this ensures failures point to the call site, not the helper.

---

## 2. Package Design

### Flat structure

Keep packages flat. The project structure is:

```
pkg/registrar/   — Registrar interface, sub-interfaces, types, request types, errors
pkg/mock/        — EPP and RRI mock servers
pkg/runner/      — YAML scenario runner
pkg/config/      — Config loading and validation
pkg/report/      — Report formatters
```

Do not create `pkg/registrar/interfaces/`, `pkg/registrar/types/`, or similar sub-packages. One level of nesting under `pkg/` is the maximum.

### One concept per file

Inside a package, separate concerns into distinct files:

- `registrar.go` — interface declarations
- `types.go` — result and request types
- `errors.go` — error types
- `request.go` — request structs (if large enough to split from types.go)
- `doc.go` — package-level godoc comment (required for packages with more than three files)

### No utility packages

Do not create `utils/`, `helpers/`, `common/`, or `shared/` packages. Put shared code in the package that owns the concept. If something is needed by two packages and has no natural home, reconsider the design.

---

## 3. Error Handling

### Always wrap with %w

```go
// CORRECT — preserves the error chain for errors.Is / errors.As
return nil, fmt.Errorf("domain check %s: %w", name, err)

// WRONG — destroys the error chain
return nil, fmt.Errorf("domain check %s: %v", name, err)
```

### Never discard errors

```go
// CORRECT
if err := conn.Close(); err != nil {
    // log or wrap — never silently drop
}

// WRONG
conn.Close() // dropped error

// WRONG
_ = conn.Close() // explicitly discarded outside defer chains
```

Exception: `_ = conn.Close()` is acceptable inside `defer` when the return value is already captured upstream and a second error would be noise. Always add a comment explaining why.

### Typed errors for protocol failures

Return `*EPPError` (pointer, not value) for EPP protocol errors. Callers use `errors.As`:

```go
var eppErr *EPPError
if errors.As(err, &eppErr) && eppErr.Code == 2302 {
    // object already exists — RFC 5730 §3
}
```

The second argument to `errors.As` must be `**EPPError` (a pointer to a pointer). Declare:
```go
var eppErr *EPPError        // nil pointer of type *EPPError
errors.As(err, &eppErr)    // pass address-of — correct
```

Never pass `&EPPError{}` directly; that creates a non-nil pointer that cannot be set by `errors.As`.

### Return nil, err not nil, nil

When an operation fails, return the zero value of the success type and a non-nil error. Never return `nil, nil` to signal failure — it forces callers to check both returns and is ambiguous.

---

## 4. Constructor Patterns

### Explicit New functions

Every type that holds state or dependencies gets an explicit constructor:

```go
// CORRECT
func NewEPPAdapter(addr string, cfg config.RegistrarConfig, opts ...Option) (*EPPAdapter, error) {
    // validate, initialise
}

// WRONG — field assignment callers must know internal structure
adapter := &EPPAdapter{host: "...", port: 700}
```

### No init() side effects

`init()` is permitted only for package-level invariant checks (e.g., verifying a compile-time constraint at startup). Never use `init()` to:
- Register global handlers
- Dial network connections
- Read files or environment variables
- Modify package-level variables

### Functional options for optional configuration

```go
type Option func(*EPPAdapter)

func WithTimeout(d time.Duration) Option {
    return func(a *EPPAdapter) { a.dialTimeout = d }
}
```

### Inject all dependencies

If a type needs a logger, clock, or other collaborator, pass it through the constructor. Never reach for a package-level global from inside a method. This is the single rule that makes tests possible without mocking global state.

---

## 5. Testability

### Constructor injection, not global singletons

```go
// CORRECT — test can pass a mock registrar
func TestDomainLifecycle(t *testing.T) {
    reg := newStubRegistrar(t)
    runner := runner.New(reg)
    ...
}

// WRONG — test cannot substitute the real registrar
func TestDomainLifecycle(t *testing.T) {
    runner := runner.New(globalRegistrar) // package-level global
    ...
}
```

### Table-driven tests

Use `t.Run(tc.name, ...)` subtests for all cases that share setup:

```go
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {"valid code 1000", "1000", 1000, false},
    {"invalid code", "abc", 0, true},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        // use tc.input, tc.want, tc.wantErr
    })
}
```

### Mock server lifecycle with t.Cleanup

```go
func newMockEPPServer(t *testing.T) *MockServer {
    t.Helper()
    srv := startMockServer(t)
    t.Cleanup(func() { srv.Close() })
    return srv
}
```

Never use `defer srv.Close()` at the test body level — `t.Cleanup` runs at test teardown even when the test calls `t.Fatal`, ensuring goroutines are not leaked.

### require vs assert

- Use `require` for preconditions that make subsequent assertions meaningless if they fail:
  ```go
  require.NoError(t, err)     // stop immediately if login failed
  assert.Equal(t, 1000, code) // non-fatal assertion on a field
  ```
- Use `assert` for independent assertions in the same test.

### Goroutine leak detection

Add `goleak.VerifyTestMain(m)` to `TestMain` in every package that starts goroutines:

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

This is mandatory in `pkg/mock/` (Phase 2). Any package that starts goroutines must add this.

---

## 6. Comment Policy

### Never restate the code

```go
// WRONG — restates the obvious
// increment i
i++

// WRONG — restates the method name
// CheckDomain checks a domain.
func (a *EPPAdapter) CheckDomain(...) {}
```

### Explain WHY, not WHAT

```go
// CORRECT — explains the non-obvious reason
// EPP result code 2302 means "object already exists" (RFC 5730 §3).
// We treat this as a non-error for idempotent create operations.
if eppErr.Code == 2302 { return existing, nil }

// CORRECT — calls out the invariant
// nbio/xml must be used instead of encoding/xml because Go's marshaler
// drops namespace prefixes (domain:create → ns0:create), which EPP
// registrars reject. See: https://github.com/golang/go/issues/48821
import "github.com/nbio/xml"
```

### Exported symbols need godoc

Every exported type, function, method, and constant needs a godoc comment starting with the symbol name:

```go
// EPPError carries an EPP result code and associated message from the server.
// Callers use errors.As to extract and inspect the code:
//
//   var eppErr *EPPError
//   if errors.As(err, &eppErr) && eppErr.Code == 2302 { ... }
type EPPError struct { ... }
```

### Package doc

Packages with more than three files get a `doc.go` with a package-level comment explaining the package's purpose and its main types.

---

## 7. Context Usage

### context.Context is always the first parameter

Any function that performs I/O, blocks, or can be cancelled takes `context.Context` as its first argument:

```go
// CORRECT
func (a *EPPAdapter) CheckDomain(ctx context.Context, names ...string) ([]DomainResult, error)

// WRONG — no way to cancel or time out
func (a *EPPAdapter) CheckDomain(names ...string) ([]DomainResult, error)
```

### Never store a Context in a struct

```go
// WRONG — makes it impossible to control cancellation per-call
type EPPAdapter struct {
    ctx context.Context
    ...
}

// CORRECT — context flows through the call stack
func (a *EPPAdapter) Login(ctx context.Context) error { ... }
```

### Context for tests

- Unit tests: `context.Background()` — no timeout needed for in-process mock servers.
- Integration tests: `context.WithTimeout(ctx, 30*time.Second)` — OT&E endpoints can be slow.

---

## 8. Build Tags

All test files in this project carry either `//go:build unit` or `//go:build integration`. The only exception is compile-time assertion files (e.g., `registrar_test.go` containing `var _ Registrar = (*stubRegistrar)(nil)` with zero test functions).

```go
//go:build unit

package registrar_test
// — blank line between the build tag and package declaration is required —
```

The `//go:build` directive must be the first non-blank line in the file, followed by a blank line, then the `package` declaration. The old `// +build` form is deprecated — do not use it in new files.

| Command | Which tests run |
|---------|----------------|
| `go test ./...` | Only untagged files (compile-time assertions only) |
| `go test -tags unit ./...` | Unit files + untagged files |
| `go test -tags integration ./...` | Integration files + untagged files |

---

## 9. EPP-Specific Rules (Critical — Encode in Every Adapter)

These rules encode known failure modes documented in research. Violating them produces bugs that are difficult to diagnose against real registrar OT&E environments.

### XML namespace prefixes

Use `github.com/nbio/xml` instead of `encoding/xml` for all EPP XML marshaling. Go's built-in marshaler drops namespace prefixes (`domain:create` becomes `ns0:create`), which EPP registrars reject. This is a known upstream Go issue (#48821) with no fix scheduled.

### EPP frame framing (RFC 5734)

The 4-byte big-endian length prefix in an EPP frame **includes the 4 bytes of the header itself**. A 100-byte XML payload requires a length field of 104, not 100. This off-by-four error is the most common EPP implementation bug.

```
Frame format:  [4-byte total length] [XML payload]
Total length = 4 (header) + len(payload)
```

Always test `ReadFrame`/`WriteFrame` helpers against an RFC-exact byte sequence before writing any other protocol code.

### DENIC RRI password hashing

DENIC RRI transmits passwords as MD5 hashes, not plaintext. The `go-rriclient` library handles this, but do not bypass it or substitute your own auth logic.

### No cgo dependencies

Never import a cgo-requiring package (e.g., `github.com/mattn/go-sqlite3`, `github.com/lib/pq` with cgo). The project must compile with `CGO_ENABLED=0`. All selected libraries are pure Go; verify before adding any new dependency.

### Config env override limitation

Viper's `AutomaticEnv` works for statically known struct keys. For the `map[string]RegistrarConfig` keyed by registrar name, env overrides require explicit `BindEnv` calls per field. The workaround for CI: supply a minimal `epp-test.yaml` naming the registrar blocks and override only credential fields via environment variables.

---

*This document is versioned with the repository. Update it when a new convention is established; do not leave tacit conventions undocumented.*
