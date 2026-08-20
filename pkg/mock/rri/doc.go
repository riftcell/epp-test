// Package rri provides an in-process DENIC RRI mock server for unit testing.
//
// The mock server listens on a random TCP port, handles the DENIC line-oriented
// key-value text protocol over a 4-byte-framed TCP connection, and enforces login
// state per connection.
//
// Wire format note: RRI uses 4-byte big-endian framing where the length field
// encodes the payload length ONLY (not including the 4-byte header). This is
// the inverse of EPP's RFC 5734 framing (which includes the header in the count).
// See ReadRRIFrame and WriteRRIFrame for the exact implementation.
//
// Scripted response API (identical to pkg/mock/epp):
//
//	srv := rri.NewMockRRIServer(t)
//	srv.Expect <- []byte("RESULT: success\nSTID: 123\n")
//	frame := <-srv.Received
//
// Two transports are available behind the same MockRRIServer type and
// scripting API: NewMockRRIServer listens on plain TCP (for clients whose
// dial hook accepts a generic connection interface), and NewMockRRIServerTLS
// listens over TLS with a self-signed certificate (for clients whose dial
// hook requires a concrete *tls.Conn, such as wcs-externalapi's rri.Client).
package rri
