# How-To: Hosts

This guide explains how to test host (nameserver) lifecycle operations using the EPP Test Framework. It is anchored to the `host_lifecycle` conformance scenario (`scenarios/conformance/host_lifecycle.yaml`), which exercises RFC 5732 §3.2 operations: create, info, update (add glue record), and delete.

See [Getting Started](getting-started.md) for setup instructions and [Config Reference](config-reference.md) for credential configuration.

## Registrar Interface Methods

The `Registrar` interface exposes host operations through `HostManager`:

| Method | Description |
|--------|-------------|
| `CheckHost(ctx, names...)` | Reports whether one or more host names are available |
| `InfoHost(ctx, name)` | Returns the full host object (name + glue addresses) |
| `CreateHost(ctx, req)` | Registers a new host object with optional glue IPv4/IPv6 addresses |
| `UpdateHost(ctx, req)` | Adds or removes glue addresses (or host statuses) |
| `DeleteHost(ctx, name)` | Removes a host object |

All methods accept `context.Context` as the first argument.

`HostCreateRequest` carries: `Name` (FQDN), `Addrs` (initial glue IPv4/IPv6 addresses). `HostUpdateRequest` carries: `Name`, `AddAddrs`, `RemAddrs`, `AddStatuses`, `RemStatuses`.

### Subordinate vs. External Hosts

A **subordinate host** has a name that falls under a registered domain (e.g., `ns1.example.com` when `example.com` is in the registry). Subordinate hosts require glue addresses. An **external host** is managed outside the registry and does not require glue records. The conformance scenario uses a subordinate host to exercise glue-address management.

## Conformance Scenario

The `host_lifecycle` scenario (`scenarios/conformance/host_lifecycle.yaml`) runs against `internetx`, `nicat`, and `eurid`:

```yaml
name: host_lifecycle
rfc: "RFC 5732 §3.2"
matrix: [internetx, nicat, eurid]

steps:
  - name: create_host
    op: create_host
    params:
      name: ns1.conformance.example
      addrs: ["192.0.2.1"]
    expect:
      code: 1000

  - name: info_host
    op: info_host
    params:
      name: ns1.conformance.example
    expect:
      code: 1000
      fields:
        Name: ns1.conformance.example

  - name: update_host
    op: update_host
    params:
      name: ns1.conformance.example
      add_addrs: ["192.0.2.2"]
    expect:
      code: 1000

  - name: delete_host
    op: delete_host
    params:
      name: ns1.conformance.example
    expect:
      code: 1000
```

The `create_host` step creates `ns1.conformance.example` with a single glue address `192.0.2.1`. The `update_host` step adds a second glue address `192.0.2.2` — this is the standard pattern for adding redundant nameserver glue without removing the existing address.

## What to Assert

**`create_host`** — Assert `code: 1000`. The runner registers a cleanup to call `DeleteHost` at test end. Initial glue addresses are set via the `addrs` parameter.

**`info_host`** — Assert `code: 1000` and check `Name: ns1.conformance.example`. The `HostResult` also carries `Addrs` (the glue records), `Status` values, and `ROID`. You can add field assertions to verify glue addresses were stored correctly.

**`update_host`** — Assert `code: 1000`. The `add_addrs` parameter appends glue addresses; `rem_addrs` removes them. To verify the update took effect, follow with an `info_host` step and assert the updated address list.

**`delete_host`** — Assert `code: 1000`. Attempting to delete a host that is still delegated to a domain returns EPP code `2305` (object association prohibits operation). Remove all domain delegations before deleting.

## Running the Scenario

```sh
# Unit test (mock server, offline)
go test -tags unit -run TestHostLifecycle ./scenarios/conformance/

# Integration test against EURid (requires credentials)
EPP_REGISTRARS_EURID_PASSWORD=secret \
  go test -tags integration -run TestHostLifecycle/eurid ./scenarios/conformance/
```
