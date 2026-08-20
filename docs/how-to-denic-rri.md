# How-To: DENIC RRI

This guide explains how to test DENIC domain operations using the EPP Test Framework. DENIC uses a proprietary text-based TCP protocol called **RRI** (Registry Registrar Interface) instead of EPP XML. The framework bridges RRI to the standard `Registrar` interface via `DENICAdapter`, so scenario files look identical to EPP scenarios — only the registrar key and operation mapping differ.

It is anchored to the `denic_rri` conformance scenario (`scenarios/conformance/denic_rri.yaml`), which covers TRANSIT (domain transfer request), AUTHINFO1/AUTHINFO2 (auth code generation), domain info, and delete.

See [Getting Started](getting-started.md) for setup instructions and [Config Reference](config-reference.md) for credential configuration.

## RRI Protocol Background

RRI is a line-oriented key:value protocol over plain TCP (not TLS from the client's perspective — DENIC terminates TLS at their gateway). Each RRI request and response consists of a version line, an action line, and zero or more `key:value` pairs, terminated by a blank line. There is no XML.

**Password hashing:** DENIC RRI requires passwords to be transmitted as lowercase MD5 hex digests, not plaintext. The `RRIAdapter` calls `md5Hex(password)` before sending the `LOGIN` command. Do not bypass this — the `go-rriclient` library handles the hash, but the adapter pre-hashes to prevent double-hashing on reconnect.

**`go-rriclient` integration:** `DENICAdapter` wraps `github.com/DENICeG/go-rriclient/pkg/rri` v1.26.0 (official DENIC library, MIT license). `NoAutoRetry: true` is set to prevent the library from reconnecting with an already-hashed password.

## Registrar Interface Mapping

`DENICAdapter` translates `Registrar` method calls to RRI commands:

| Registrar Method | RRI Command |
|-----------------|-------------|
| `Login` | `LOGIN` (password is MD5-hashed before sending) |
| `Logout` | `LOGOUT` |
| `Ping` | no-op (RRI has no hello/keepalive) |
| `CreateDomain` | `CREATE` |
| `InfoDomain` | `INFO` |
| `DeleteDomain` | `DELETE` |
| `TransferDomain` (op=request) | `TRANSIT` |
| `TransferDomain` (op=authinfo1) | `CREATE-AUTHINFO1` |
| `TransferDomain` (op=authinfo2) | `CREATE-AUTHINFO2` |
| `TransferDomain` (other ops) | `EPPError{Code: 2101}` (not implemented) |

Contact and host operations return `EPPError{Code: 2101}` for DENIC — RRI manages contacts and hosts server-side; registrars do not create them directly.

## Conformance Scenario

The `denic_rri` scenario (`scenarios/conformance/denic_rri.yaml`) runs only against the `denic` registrar:

```yaml
name: denic_rri
rfc: "DENIC RRI domain operations"
matrix: [denic]

steps:
  - name: create
    op: create_domain
    params:
      name: conformance.de
      registrant: "DENIC-001"
      period: 1
    expect:
      code: 1000

  # op=request maps to TRANSIT per RRIAdapter.TransferDomain (STATE 03-04).
  - name: transit
    op: transfer_domain
    params:
      name: conformance.de
      op: request
      auth_info: "x"
    expect:
      code: 1000

  # op=authinfo1 maps to CREATE-AUTHINFO1 (generates auth code with 5-day expiry).
  - name: authinfo1
    op: transfer_domain
    params:
      name: conformance.de
      op: authinfo1
    expect:
      code: 1000

  # op=authinfo2 maps to CREATE-AUTHINFO2 (incoming registrar redeems auth code).
  - name: authinfo2
    op: transfer_domain
    params:
      name: conformance.de
      op: authinfo2
    expect:
      code: 1000

  - name: info
    op: info_domain
    params:
      name: conformance.de
    expect:
      code: 1000

  - name: delete
    op: delete_domain
    params:
      name: conformance.de
    expect:
      code: 1000
```

### TRANSIT

`op: request` maps to the RRI `TRANSIT` command, which initiates a domain transfer at DENIC. Unlike EPP's `transfer request` (which notifies the losing registrar), TRANSIT is an immediate operation — the domain is transferred as soon as TRANSIT is sent. The `auth_info` parameter carries the authorization code required by DENIC for the outgoing registrar.

### AUTHINFO1 and AUTHINFO2

DENIC's two-step auth-code mechanism works as follows:

1. **AUTHINFO1** (`op: authinfo1`): The current registrar calls `CREATE-AUTHINFO1`, which generates a temporary auth code valid for five days. The code is shared with the registrant.
2. **AUTHINFO2** (`op: authinfo2`): The incoming registrar calls `CREATE-AUTHINFO2` with the auth code, completing the transfer.

These map to `transfer_domain` with `op: authinfo1` and `op: authinfo2` respectively. No EPP equivalent exists — this is a DENIC-specific protocol extension.

## What to Assert

**`create_domain`** — Assert `code: 1000`. DENIC does not use a period/year model; the `period: 1` parameter is accepted but the RRI command does not include it.

**`transit`** — Assert `code: 1000`. A successful TRANSIT confirms the domain was accepted by DENIC for transfer.

**`authinfo1`** — Assert `code: 1000`. The response from `CREATE-AUTHINFO1` carries the generated auth code in the result data. Capture it with `${authinfo1.AuthInfo}` for use in the `authinfo2` step.

**`authinfo2`** — Assert `code: 1000`. Confirms the incoming registrar successfully claimed the auth code.

**`info_domain`** — Assert `code: 1000`. Verify domain fields: `Name`, `ROID`, holder handle, registered date.

**`delete_domain`** — Assert `code: 1000`. DENIC deletes `.de` domains immediately; no redemption grace period applies via RRI.

## Running the Scenario

```sh
# Unit test (mock RRI server, offline)
go test -tags unit -run TestDENICRRI ./scenarios/conformance/

# Integration test against DENIC OT&E (requires credentials)
EPP_REGISTRARS_DENIC_PASSWORD=secret \
  go test -tags integration -run TestDENICRRI/denic ./scenarios/conformance/
```

## Mock RRI Server

Unit tests use `pkg/mock/rri.MockRRIServer`, which listens on `127.0.0.1:0` (OS-assigned port) and replays scripted RRI responses. `t.Cleanup` handles teardown automatically — do not call `Close` manually. The type is constructed by one of two constructors that differ only in transport.

**Constructor comparison:**

| Constructor | Transport | Use when |
|-------------|-----------|----------|
| `NewMockRRIServer(t)` | Plain TCP | The RRI client dials plain TCP, or its dial hook is an injection point typed as a generic connection interface (satisfiable by a plain `net.Conn`). This is what the framework's own `DENICAdapter` / `RRIAdapter` tests use. |
| `NewMockRRIServerTLS(t)` | TLS (self-signed cert, `MinVersion` TLS 1.2) | The RRI client's dial hook is typed to return a concrete `*tls.Conn` — for example `wcs-externalapi`'s `rri.Client` and its `TLSDialHandler`. Such a hook cannot be pointed at a plain-TCP listener. |

**Decision rule:** if your client's dial hook can accept any `net.Conn`, use `NewMockRRIServer`; if it is typed to a concrete `*tls.Conn`, use `NewMockRRIServerTLS`.

Everything else — `Expect`/`Received` scripting, `AddUser` login simulation (including the MD5-hex password requirement described above), `SetDelay`, `DropConnection`, and `Addr()` — behaves identically on both. Only the transport differs.

**Certificate note:** the TLS certificate is self-signed and generated per server; no CA pool is provided. DENIC RRI clients dial with `InsecureSkipVerify`, so nothing validates the certificate chain — this is intentional, not an oversight.

```go
srv := rri.NewMockRRIServerTLS(t)
srv.AddUser("DENIC-Test", "s3cr3t")
conn, err := tls.Dial("tcp", srv.Addr(), &tls.Config{InsecureSkipVerify: true})
```

`WithClientConfig` injects a plain-TCP dialer in the framework's test path. Production `DENICAdapter` uses the default TLS dialer from `go-rriclient`.
