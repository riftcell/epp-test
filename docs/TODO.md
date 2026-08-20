# TODO / Known Tech Debt

## OT&E Integration Tests

Provision real OT&E/sandbox accounts for InternetX, NiCAT, and EURid and run `go test -tags integration` against each endpoint. The adapters are wired and the conformance scenarios exist — accounts are the only missing piece.

## In-Process Mock Fault Modes

`MockEPPServer` (pkg/mock/epp) currently lacks `connect_delay` and `hang` modes. These are available in the standalone `cmd/mock-epp-server` via `--connect-delay` and `--login-mode=hang`, but not accessible to unit tests that use the in-process mock directly.

## RRI Logical Mismatch Fault

The standalone `cmd/mock-rri-server` has no `--fault-mismatch` flag because the standard `RESULT: success` KV response carries no domain name field to substitute. Implementing this would require defining a custom extended success response shape (e.g. adding a `domain-name:` field) and documenting it as a non-standard extension.

## `nbio/xml` Stabilization

`github.com/nbio/xml` has no stable tagged release — the project uses a pseudo-version (`v0.0.0-20260302224236-9f64bb3b5a9e`). Options: track the upstream Go issue [#48821](https://github.com/golang/go/issues/48821) for a stdlib fix, or vendor the package to reduce supply-chain risk.
