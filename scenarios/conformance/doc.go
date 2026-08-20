// Package conformance provides the prebuilt EPP/RRI conformance scenario library.
//
// This package lives at scenarios/conformance/ at the repository root (not under pkg/)
// per decision D-05 (CONTEXT.md): scenario YAML files are user-facing test specs that
// operators can read and extend, not internal library code. Placing them at the repo root
// makes them immediately discoverable.
//
// The seven YAML scenario files cover the full EPP/RRI operation matrix:
//
//   - domain_lifecycle.yaml  — CONF-01: check/create/info/update/renew/transfer/delete (RFC 5731)
//   - contact_lifecycle.yaml — CONF-02: create/info/update/delete with domain association (RFC 5733)
//   - host_lifecycle.yaml    — CONF-03: create/info/update/delete (RFC 5732)
//   - poll_lifecycle.yaml    — CONF-04: wait/poll/ack/verify-empty (RFC 5730 §2.9.2.3)
//   - denic_rri.yaml         — CONF-05: DENIC RRI create/transit/authinfo1/authinfo2/info/delete
//   - fault_tolerance.yaml   — CONF-06: malformed-frame no-panic (RFC 5730 fault tolerance)
//   - negative_tests.yaml    — CONF-07: expected error codes 2302/2303/2201/2306 (RFC 5730 §3)
//
// Each scenario file carries an rfc: header citing the RFC clause it validates (CONF-09).
// Scenarios declare a matrix: list for registrar-agnostic execution (CONF-08, D-06) and
// may include an overrides: section for registrar-specific parameter deep-merge (D-07).
//
// The test harness (conformance_test.go) runs each scenario against a mock server and
// flushes all report formats to TEST_REPORT_DIR after m.Run() (RPT-05, D-09).
package conformance
