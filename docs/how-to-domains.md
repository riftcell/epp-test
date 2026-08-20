# How-To: Domains

This guide explains how to test domain lifecycle operations using the EPP Test Framework. It is anchored to the `domain_lifecycle` conformance scenario (`scenarios/conformance/domain_lifecycle.yaml`), which exercises the full RFC 5731 §3.2 operation set: check, create, info, update, renew, transfer, and delete.

See [Getting Started](getting-started.md) for setup instructions and [Config Reference](config-reference.md) for credential configuration.

## Registrar Interface Methods

The `Registrar` interface exposes domain operations through `DomainManager`, which composes three sub-interfaces:

| Method | Sub-interface | Description |
|--------|--------------|-------------|
| `CheckDomain(ctx, names...)` | `DomainChecker` | Reports availability for one or more names |
| `InfoDomain(ctx, name)` | `DomainReader` | Returns the full domain object |
| `CreateDomain(ctx, req)` | `DomainWriter` | Registers a new domain |
| `UpdateDomain(ctx, req)` | `DomainWriter` | Adds/removes nameservers, contacts, statuses |
| `DeleteDomain(ctx, name)` | `DomainWriter` | Removes a domain object |
| `RenewDomain(ctx, name, years)` | `DomainWriter` | Extends the registration period |
| `TransferDomain(ctx, req)` | `DomainWriter` | Initiates, approves, cancels, rejects, or queries a transfer |

All methods accept `context.Context` as the first argument for timeout and cancellation.

## Conformance Scenario

The `domain_lifecycle` scenario (`scenarios/conformance/domain_lifecycle.yaml`) runs against `internetx`, `nicat`, and `eurid`. It first creates a contact to use as registrant, then exercises the full domain operation sequence:

```yaml
name: domain_lifecycle
rfc: "RFC 5731 §3.2"
matrix: [internetx, nicat, eurid]

steps:
  - name: check
    op: check_domain
    params:
      names: [conformance.example]
    expect:
      code: 1000
      fields:
        available: true

  - name: create_contact
    op: create_contact
    params:
      name: "Test User"
      email: "test@example.com"
      auth_info: "authTest123"
    expect:
      code: 1000

  - name: create
    op: create_domain
    params:
      name: conformance.example
      registrant: "${create_contact.ID}"
      period: 1
      auth_info: "domAuth99"
    expect:
      code: 1000

  - name: info
    op: info_domain
    params:
      name: conformance.example
    expect:
      code: 1000
      fields:
        Name: conformance.example

  - name: update
    op: update_domain
    params:
      name: conformance.example
      add_statuses: [clientHold]
    expect:
      code: 1000

  - name: renew
    op: renew_domain
    params:
      name: conformance.example
      years: 1
    expect:
      code: 1000

  - name: transfer
    op: transfer_domain
    params:
      name: conformance.example
      op: query
    expect:
      code: 1000

  - name: delete
    op: delete_domain
    params:
      name: conformance.example
    expect:
      code: 1000

overrides:
  nicat:
    steps:
      create:
        params:
          extensions:
            verification_level: owner
```

### Variable Interpolation

The `${create_contact.ID}` token in the `create` step is resolved at runtime from the result of the `create_contact` step. The runner captures each step's result after execution; subsequent steps can reference any captured field using `${step_name.FieldName}` syntax.

### Per-Registrar Overrides

The `overrides.nicat.steps.create` block deep-merges additional params onto the base `create` step when running against NiCAT. The `verification_level: owner` extension is required by NiCAT's `at-ext-verification-1.0` extension but ignored by other registrars.

## What to Assert

**`check_domain`** — Assert `code: 1000` and `available: true` for a domain that should not exist yet. If the domain already exists from a previous failed test run, the runner's automatic cleanup (via `t.Cleanup`) will delete it at the end. You can also assert `available: false` for a known-registered domain.

**`create_domain`** — Assert `code: 1000`. The runner automatically registers a cleanup closure that calls `DeleteDomain` when the test ends, so test isolation is maintained even if later steps fail.

**`info_domain`** — Assert `code: 1000` and check specific fields: `Name` (the FQDN), `ROID` (registry object ID), `Status` values. Field comparison is case-insensitive.

**`update_domain`** — Assert `code: 1000`. If you add a status like `clientHold`, you can verify it with a subsequent `info_domain` step and `fields: Status: clientHold`.

**`renew_domain`** — Assert `code: 1000`. The response carries the new expiry date in the `ExpiryDate` field.

**`transfer_domain`** — For `op: query`, assert `code: 1000`. For `op: request` (DENIC TRANSIT), see [How-To: DENIC RRI](how-to-denic-rri.md).

**`delete_domain`** — Assert `code: 1000`. After deletion, a subsequent `check_domain` should return `available: true`.

## Running the Scenario

```sh
# Unit test (mock server, offline)
go test -tags unit -run TestDomainLifecycle ./scenarios/conformance/

# Integration test against a specific registrar (requires credentials)
EPP_REGISTRARS_INTERNETX_PASSWORD=secret \
  go test -tags integration -run TestDomainLifecycle/internetx ./scenarios/conformance/
```

For the full matrix, use `runner.RunMatrix` in your test file:

```go
//go:build unit

func TestDomainLifecycle(t *testing.T) {
    runner.RunMatrix(t, "domain_lifecycle.yaml", regs)
}
```
