package epp

// MalformedFrameFault is a sentinel value for the MockEPPServer.Expect channel.
// When dequeued, the mock writes a frame with an incorrect length prefix,
// testing that the EPP client detects and rejects malformed frames (MOCK-05).
//
// Usage:
//
//	srv.Expect <- epp.MalformedFrameFault{}
type MalformedFrameFault struct{}

// WrongResultCodeFault is a sentinel value for the MockEPPServer.Expect channel.
// When dequeued, the mock returns a syntactically valid EPP XML envelope containing
// the specified unexpected result code, testing that the client handles unexpected
// codes without panicking (MOCK-06).
//
// Usage:
//
//	srv.Expect <- epp.WrongResultCodeFault{Code: 2302}
type WrongResultCodeFault struct {
	// Code is the EPP result code to embed in the response envelope.
	// See RFC 5730 §3 for standard codes; use non-standard values to test
	// client handling of unknown codes.
	Code int
}
