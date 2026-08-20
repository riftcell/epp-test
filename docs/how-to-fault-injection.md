# How-To: Fault Injection

The standalone mock servers (`mock-epp-server` and `mock-rri-server`) support configurable fault simulation to verify that your EPP or RRI client library handles failure conditions correctly. Faults are configured via CLI flags for simple one-liner cases, or via a YAML fault profile for multi-rule scenarios — and explicitly-set CLI flags always override the corresponding YAML values (D-01).

## Quick Start

```bash
# Fail every login attempt (test client EPP 2200 / RRI RESULT:failed error handling)
go run ./cmd/mock-epp-server --login-mode=always

# Add 2-second delay before greeting (test client connect timeout)
go run ./cmd/mock-epp-server --connect-delay=2s

# Load a YAML fault profile
go run ./cmd/mock-epp-server --fault-profile=fault-profile.yaml
```

---

## CLI Flags Reference

### EPP Server (8 fault flags)

| Flag | Default | Description |
|------|---------|-------------|
| `--fault-profile=<path>` | `""` | Load all faults from YAML file; explicitly-set flags override YAML values |
| `--connect-delay=<dur>` | `""` | Sleep before TLS greeting (e.g. `2s`, `500ms`) |
| `--login-mode=<mode>` | `""` | `always` \| `flap` \| `hang` \| `disconnect` |
| `--login-fail-count=<N>` | `0` | Number of logins to fail before allowing (flap mode) |
| `--response-delay=<dur>` | `""` | Sleep before every non-login response (e.g. `200ms`) |
| `--disconnect-after=<N>` | `0` | Close connection after N operations; `0` = disabled |
| `--malformed-frame=<kind>` | `""` | `length_overflow` \| `invalid_xml` \| `garbage` (EPP only) |
| `--fault-mismatch=<op>` | `""` | Op substring triggering a mismatch response (EPP only, e.g. `domain:create`) |

### RRI Server (6 fault flags)

| Flag | Default | Description |
|------|---------|-------------|
| `--fault-profile=<path>` | `""` | Load all faults from YAML file; explicitly-set flags override YAML values |
| `--connect-delay=<dur>` | `""` | Sleep before first RRI frame read (e.g. `2s`, `500ms`) |
| `--login-mode=<mode>` | `""` | `always` \| `flap` \| `hang` \| `disconnect` |
| `--login-fail-count=<N>` | `0` | Number of logins to fail before allowing (flap mode) |
| `--response-delay=<dur>` | `""` | Sleep before every non-login response (e.g. `200ms`) |
| `--disconnect-after=<N>` | `0` | Close connection after N operations; `0` = disabled |

Note: `--malformed-frame` and `--fault-mismatch` are EPP-only and are **not available on the RRI server**. RRI is newline-delimited text with no binary framing, so frame-level corruption faults do not apply. RRI `RESULT: success` responses have no resource name field, so logical mismatch faults do not apply (D-07, D-08).

---

## Fault Categories

### 1. Connection Delay

**Simulates:** slow TLS handshake, network congestion, or an overloaded server that is slow to accept new connections.

**Useful for:** verifying that the client sets a connect/dial timeout and returns an error rather than blocking indefinitely.

```bash
# EPP: 2-second sleep before the TLS greeting frame
go run ./cmd/mock-epp-server --connect-delay=2s

# RRI: 500ms sleep before the first ReadRRIFrame call
go run ./cmd/mock-rri-server --connect-delay=500ms
```

**Client observes:** if the client's dial/connect timeout is shorter than `--connect-delay`, the connection attempt returns a deadline-exceeded or `i/o timeout` error before any protocol data is exchanged.

---

### 2. Login Failures

**Simulates:** wrong credentials, temporary account lockout, an authentication service outage, or a server under rolling restart.

**Useful for:** verifying retry logic, exponential backoff, and correct error propagation for each login failure mode.

| Mode | Server behavior | Client observes |
|------|----------------|-----------------|
| `always` | Every login returns EPP 2200 / RRI `RESULT: failed` | Auth error on every attempt |
| `flap` | First N logins fail (2200 / failed), then succeed | N failures followed by success |
| `hang` | Server reads the login frame and never responds | Read deadline/timeout error |
| `disconnect` | Server closes TCP immediately after receiving the login frame | EOF / connection reset |

```bash
go run ./cmd/mock-epp-server --login-mode=always
go run ./cmd/mock-epp-server --login-mode=flap --login-fail-count=3
go run ./cmd/mock-epp-server --login-mode=hang
go run ./cmd/mock-epp-server --login-mode=disconnect
```

Note: `hang` mode requires the client to have a read deadline set. Without one, the client blocks indefinitely waiting for a response that never arrives.

---

### 3. Abrupt Disconnection

**Simulates:** server crash, network drop, or resource exhaustion after a partial session.

**Useful for:** verifying the client detects EOF, does not hang, and returns a clear error to the caller rather than silently retrying or panicking.

| Variant | How to trigger | Server behavior |
|---------|---------------|-----------------|
| `pre-greeting` | YAML `disconnect_at: "pre-greeting"` | Accept TCP connection, then close before sending any protocol data |
| `post-greeting` | YAML `disconnect_at: "post-greeting"` | Send EPP greeting (or accept first RRI frame), then close |
| `after-N-ops` | `--disconnect-after=N` | Close when the operation count exceeds N; the (N+1)th operation gets no response |
| `on-specific-op` | YAML `per_op: [{match: "...", disconnect: true}]` | Close when a matching operation arrives |

```bash
# Close after 3 operations (ops 1-3 succeed; op 4 gets EOF)
go run ./cmd/mock-epp-server --disconnect-after=3

# Close on domain:create (via YAML per_op rule)
go run ./cmd/mock-epp-server --fault-profile=disconnect-on-create.yaml
```

Note: `pre-greeting` and `post-greeting` disconnect modes must be configured via a YAML fault profile — they are not available as CLI flags.

---

### 4. Response Delays

**Simulates:** a slow backend database, registry-side throttling, or a high-latency network path between the client and registry.

**Useful for:** verifying the client sets per-operation read deadlines and times out cleanly; also useful for throughput benchmarking under simulated latency.

Global delay applies to every non-login response. Per-op delays from the YAML `per_op` list are additive on top of the global delay (D-06): if the global is 100ms and a matching per-op rule adds 400ms, that operation waits 500ms total.

```bash
# 200ms before every response
go run ./cmd/mock-epp-server --response-delay=200ms

# 100ms global + extra 400ms only for domain:create (500ms total for that op)
# Requires a YAML profile: per_op: [{match: "domain:create", delay: "400ms"}]
go run ./cmd/mock-epp-server --response-delay=100ms --fault-profile=slow-create.yaml
```

---

### 5. Malformed EPP Frames (EPP only)

**Simulates:** a server implementation bug, wire-level data corruption, or an unexpected protocol version.

**Useful for:** verifying the client's frame parser handles corrupt data without panicking, hanging, or silently returning wrong results. The malformed frame is injected into the **first non-login response only**; all subsequent responses are normal.

| Kind | Frame length header | Frame body | Client observes |
|------|---------------------|------------|-----------------|
| `length_overflow` | Claims N+100 bytes | N bytes only | `ReadFrame` blocks in `io.ReadFull`; client must hit read deadline |
| `invalid_xml` | Correct length | Non-parseable XML (`<not valid xml<<<`) | XML parse error |
| `garbage` | Correct length | Raw non-UTF8 bytes (`0xFF 0xFE ...`) | UTF-8 or XML decode error |

```bash
go run ./cmd/mock-epp-server --malformed-frame=length_overflow
go run ./cmd/mock-epp-server --malformed-frame=invalid_xml
go run ./cmd/mock-epp-server --malformed-frame=garbage
```

**Important for `length_overflow`:** the client must have a read deadline set. Without one, the client blocks forever waiting for bytes that never arrive, making the test hang rather than fail.

---

### 6. Logical Mismatch (EPP only)

**Simulates:** a server that responds with data for a different resource than the one requested — a real bug class observed in some registry implementations under concurrent load.

**Useful for:** verifying the client validates that the response resource name matches the request, rather than blindly trusting the response payload.

The wrong name is hardcoded to be unambiguously wrong:
- `domain:create` request → response `domain:creData` contains `mismatch-sentinel.example` (not the requested domain)
- `contact:create` request → response `contact:creData` contains `MISMATCH-C` (not the requested contact ID)

```bash
# Mismatch on all domain:create operations
go run ./cmd/mock-epp-server --fault-mismatch=domain:create

# Mismatch on contact:create (via YAML per_op rule)
go run ./cmd/mock-epp-server --fault-profile=mismatch-contact.yaml
```

---

## YAML Fault Profile

Use a YAML fault profile when you need multi-rule scenarios, want to reuse a configuration across runs, or need to configure `disconnect_at` (which has no CLI flag equivalent). The YAML schema is identical for both EPP and RRI servers.

### Complete Schema

```yaml
# fault-profile.yaml
# Load with: --fault-profile=fault-profile.yaml
# Flags explicitly set on the command line override these YAML values.

connect_delay: "2s"        # sleep before TLS greeting (EPP) or first read (RRI)
response_delay: "100ms"    # global sleep before every non-login response
login_mode: "flap"         # always | flap | hang | disconnect
login_fail_count: 3        # for flap: number of login failures before allowing

# Timing-based disconnect. Configure only via YAML (no CLI flag).
# "pre-greeting"  — close before any protocol data is sent
# "post-greeting" — close after EPP greeting is sent / after RRI connection is accepted
disconnect_at: ""

disconnect_after: 0        # close after N non-login operations; 0 = disabled
malformed_frame: ""        # EPP only: length_overflow | invalid_xml | garbage
fault_mismatch: ""         # EPP only: op substring (e.g. "domain:create")

per_op:
  - match: "domain:check"
    delay: "400ms"         # additional delay (additive with response_delay)
  - match: "domain:create"
    mismatch: true         # respond with mismatch-sentinel.example
  - match: "contact:info"
    disconnect: true       # close when this op arrives
  - match: "domain:update"
    result_code: 2400      # override result code (reserved; future use)
```

### Named Example Profiles

**`slow-login.yaml`** — simulate overloaded server before TLS handshake

```yaml
# slow-login.yaml — simulate overloaded server before TLS handshake
connect_delay: "3s"
```

Usage: `go run ./cmd/mock-epp-server --fault-profile=slow-login.yaml`

---

**`disconnect-on-create.yaml`** — drop connection on domain:create

```yaml
# disconnect-on-create.yaml — test client reconnect after mid-session drop
per_op:
  - match: "domain:create"
    disconnect: true
```

Usage: `go run ./cmd/mock-epp-server --fault-profile=disconnect-on-create.yaml`

---

**`flap-then-succeed.yaml`** — 2 login failures then allow

```yaml
# flap-then-succeed.yaml — simulate recovering authentication service
login_mode: "flap"
login_fail_count: 2
```

Usage: `go run ./cmd/mock-epp-server --fault-profile=flap-then-succeed.yaml`

---

## Combining Faults

Faults compose. You can pass multiple flags to stack multiple fault behaviors in a single server run:

```bash
go run ./cmd/mock-epp-server \
  --connect-delay=1s \
  --login-mode=flap \
  --login-fail-count=2 \
  --response-delay=100ms \
  --disconnect-after=5
```

With `--fault-profile`, individually-set flags override only the corresponding YAML keys. You can use a shared base profile and tweak one value per run:

```bash
# Use the profile but increase connect delay for this run only
go run ./cmd/mock-epp-server \
  --fault-profile=flap-then-succeed.yaml \
  --connect-delay=5s
```

---

## What to Assert in Client Tests

| Fault | Client observes | Assert |
|-------|-----------------|--------|
| `--login-mode=always` | EPP 2200 / RRI `RESULT: failed` | Error type is auth failure, not timeout |
| `--login-mode=flap --login-fail-count=2` | First 2 logins fail, 3rd succeeds | Client eventually succeeds after 2 retries |
| `--login-mode=hang` | Login read blocks until deadline | Error is i/o timeout, not auth failure |
| `--login-mode=disconnect` | EOF after login frame sent | Client detects EOF, does not panic |
| `--connect-delay=5s` (with short timeout) | Dial/connect times out | Error is timeout before any data received |
| `--disconnect-after=2` | 3rd op returns EOF | Client detects disconnect, returns error |
| YAML `disconnect_at: "pre-greeting"` | EOF immediately on connect | No greeting received; error on first read |
| `--malformed-frame=length_overflow` | First post-login read blocks until deadline | Error is i/o timeout, not parse error |
| `--malformed-frame=invalid_xml` | First post-login response is unparseable | Error is XML parse error, not panic |
| `--malformed-frame=garbage` | First post-login response is non-UTF8 | Error is decode/parse error |
| `--fault-mismatch=domain:create` | `creData` contains `mismatch-sentinel.example` | Client returns error: response name does not match requested name |
