# How-To: Polling

This guide explains how to test EPP message queue operations using the EPP Test Framework. It is anchored to the `poll_lifecycle` conformance scenario (`scenarios/conformance/poll_lifecycle.yaml`), which exercises RFC 5730 §2.9.2.3: wait for a notification, read it, assert its content, acknowledge it, and verify the queue is empty.

See [Getting Started](getting-started.md) for setup instructions and [Config Reference](config-reference.md) for credential configuration.

## Registrar Interface Methods

The `Registrar` interface exposes polling through `Poller`:

| Method | Description |
|--------|-------------|
| `PollRead(ctx)` | Retrieves the next pending server message. Returns `EPPError{Code: 1300}` when the queue is empty. |
| `PollAck(ctx, msgID)` | Acknowledges and dequeues the message with the given ID. |

Additionally, the runner provides a higher-level `wait_for_poll` op that wraps `PollRead` in a retry loop with a configurable timeout. This is the recommended way to wait for asynchronous notifications (transfer confirmations, domain expiry notices, etc.).

## Conformance Scenario

The `poll_lifecycle` scenario (`scenarios/conformance/poll_lifecycle.yaml`) runs against `internetx`, `nicat`, and `eurid`:

```yaml
name: poll_lifecycle
rfc: "RFC 5730 §2.9.2.3"
matrix: [internetx, nicat, eurid]

steps:
  # wait_for_poll waits for the next available poll message.
  # Note: match_type is omitted — the EPP adapter does not populate PollMessage.Type
  # from wire responses (Type is only available via DENIC RRI). An empty match_type
  # matches any message, which is the correct behavior for EPP scenarios.
  - name: wait_transfer
    op: wait_for_poll
    params:
      timeout: "5s"
    expect:
      code: 1000

  # Read the next pending message from the queue.
  - name: read
    op: poll
    expect:
      code: 1000

  # Acknowledge the message using the ID captured from the read step.
  - name: ack
    op: poll_ack
    params:
      id: "${read.ID}"
    expect:
      code: 1000

  # Verify the queue is empty after acknowledgement (CONF-04: queue empty check).
  - name: verify_empty
    op: poll
    expect:
      code: 1300
```

### `wait_for_poll` vs. Direct `poll`

`wait_for_poll` retries `PollRead` every 200 ms until a message arrives or `timeout` elapses. Use it when the notification may not be immediately available (e.g., waiting for a transfer approval from a losing registrar). The scenario names this step `wait_transfer` to document its purpose.

After `wait_for_poll` finds and acknowledges the message, the `read` step calls `PollRead` directly to demonstrate the manual read pattern and to capture the message ID for explicit acknowledgement.

### `match_type` — EPP vs. RRI

The `match_type` parameter is omitted in the EPP scenario. `GenericEPPAdapter` does not populate `PollMessage.Type` from EPP XML wire responses — the message type is only available via DENIC RRI (`RRIAdapter` populates it). An empty `match_type` matches any message, which is correct for EPP conformance tests. To filter by message type in RRI tests, set `match_type: "TRANSIT"` or another RRI message type.

### EPP Code 1300

`PollRead` returns `EPPError{Code: 1300}` when the message queue is empty. The `verify_empty` step expects `code: 1300`, which demonstrates how to assert that an operation returns a specific non-error code. From an EPP protocol perspective, 1300 is a success response ("Command completed successfully; no messages") — it is not an error.

## What to Assert

**`wait_for_poll`** — Assert `code: 1000`. The op succeeds when a message arrives within the timeout window. The returned `PollMessage` carries: `ID` (for ack), `Type` (RRI only), `Content` (raw message body).

**`poll` (direct read)** — Assert `code: 1000` when you expect a message to be present. The runner captures the `PollMessage` result; use `${read.ID}` to reference the message ID in a subsequent `poll_ack` step.

**`poll_ack`** — Assert `code: 1000`. After a successful ack, the acknowledged message is removed from the queue and will not be returned by subsequent `PollRead` calls.

**Queue-empty check** — Assert `code: 1300` on a `poll` step after all messages have been acknowledged. This pattern confirms that the test cleaned up all generated messages and left the queue in a known state.

## Running the Scenario

```sh
# Unit test (mock server, offline)
go test -tags unit -run TestPollLifecycle ./scenarios/conformance/

# Integration test (requires credentials and a pending poll message)
EPP_REGISTRARS_INTERNETX_PASSWORD=secret \
  go test -tags integration -run TestPollLifecycle/internetx ./scenarios/conformance/
```
