// Package mock provides in-process EPP and RRI mock servers for unit testing.
//
// EPP mock server: listens on a random TCP port, handles TLS, and delivers
// pre-configured scripted responses matched to incoming EPP command types.
//
// RRI mock server: listens on a random TCP port, handles the DENIC line-oriented
// text protocol, and enforces login state per connection.
//
// Both servers are per-test (not package-global) with t.Cleanup-based lifecycle.
// See Phase 2 for implementation.
package mock
