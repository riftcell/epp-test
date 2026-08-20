package rri

// MalformedFrameFault is a sentinel value for the MockRRIServer.Expect channel.
// When dequeued, the mock writes a frame header encoding a zero-length
// message. ReadRRIFrame explicitly rejects payloadLen == 0 ("empty message"),
// so this tests that a client detects and rejects a malformed frame instead
// of hanging or misreading a later frame as this one's body.
//
// Usage:
//
//	srv.Expect <- rri.MalformedFrameFault{}
type MalformedFrameFault struct{}
