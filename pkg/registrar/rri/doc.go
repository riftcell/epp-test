// Package rri provides RRIAdapter, the DENIC RRI client that wraps
// github.com/DENICeG/go-rriclient/pkg/rri and satisfies the registrar.Registrar
// interface.
//
// # Protocol
//
// DENIC uses a proprietary text-based line-oriented protocol called RRI (Registry
// Registrar Interface) over plain TCP. It is NOT EPP XML — all messages use a
// Key-Value format with a 4-byte big-endian length header.
//
// # MD5 Password Pre-Hashing
//
// go-rriclient does NOT hash passwords internally. The RRIAdapter pre-hashes
// the password with MD5 hex before calling client.Login so that the wire frame
// carries md5Hex(password) and never the plaintext. This matches the real DENIC
// server requirement.
//
// # NoAutoRetry
//
// client.NoAutoRetry is set to true to disable go-rriclient's internal reconnect
// logic. The reconnect code re-sends the saved password verbatim. If we passed the
// already-hashed password at Login time, an automatic reconnect would re-hash it
// again (double-hash) and authentication would fail. Disabling auto-retry prevents
// this and lets the adapter handle reconnects explicitly if needed.
//
// # Unsupported Operations
//
// DENIC RRI has no standalone host objects. Methods CheckHost, InfoHost, CreateHost,
// UpdateHost, and DeleteHost return *registrar.EPPError{Code: 2101} (command not
// implemented) instead of panicking.
package rri
