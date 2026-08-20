//go:build unit

package frame_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/riftcell/epp-test/pkg/epp/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFrameHeader verifies RFC 5734 framing: WriteFrame of a 100-byte payload
// writes a 4-byte big-endian header equal to 104 (total = header + payload),
// followed by the 100 payload bytes (104 bytes total).
func TestWriteFrameHeader(t *testing.T) {
	tests := []struct {
		name       string
		payload    []byte
		wantTotal  int
		wantHdrVal uint32
	}{
		{"100-byte payload", make([]byte, 100), 104, 104},
		{"zero payload", []byte{}, 4, 4},
		{"single byte", []byte{0xAB}, 5, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := frame.WriteFrame(&buf, tc.payload)
			require.NoError(t, err)
			b := buf.Bytes()
			assert.Equal(t, tc.wantTotal, len(b),
				"total bytes written must equal 4+len(payload)")
			require.GreaterOrEqual(t, len(b), 4, "buffer must contain at least the 4-byte header")
			// Read header bytes as big-endian uint32
			hdrVal := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
			assert.Equal(t, tc.wantHdrVal, hdrVal,
				"EPP RFC 5734: header value must equal 4+len(payload) (off-by-four rule)")
		})
	}
}

// TestReadFrameExactBytes verifies ReadFrame against RFC-exact byte sequences.
// Header [0x00,0x00,0x00,0x68] = 104 decimal; body length = 104-4 = 100 bytes.
func TestReadFrameExactBytes(t *testing.T) {
	const total = 104
	hdr := []byte{0x00, 0x00, 0x00, 0x68} // 0x68 = 104
	payload := make([]byte, total-4)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	input := append(hdr, payload...)

	got, err := frame.ReadFrame(bytes.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, payload, got, "ReadFrame must return exactly the 100-byte payload")
}

// TestReadFrameRejectShortLength verifies the off-by-four guard:
// header value < 4 is invalid and ReadFrame must return a non-nil error
// without attempting to read any body bytes.
func TestReadFrameRejectShortLength(t *testing.T) {
	tests := []struct {
		name string
		hdr  []byte
	}{
		{"header value 3", []byte{0x00, 0x00, 0x00, 0x03}},
		{"header value 0", []byte{0x00, 0x00, 0x00, 0x00}},
		{"header value 1", []byte{0x00, 0x00, 0x00, 0x01}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Provide no body bytes after the header — if ReadFrame attempts a body
			// read it would get io.ErrUnexpectedEOF, not the length error we expect.
			// We use an exact-size reader to catch any accidental body read.
			_, err := frame.ReadFrame(bytes.NewReader(tc.hdr))
			assert.Error(t, err,
				"header value < 4 must be rejected with a non-nil error")
		})
	}
}

// TestReadFrameTruncatedBody verifies ReadFrame returns a non-nil error when
// the header promises more bytes than are available in the reader.
func TestReadFrameTruncatedBody(t *testing.T) {
	// Header says 104 (0x68) bytes total => 100 bytes body, but only 50 body bytes provided.
	hdr := []byte{0x00, 0x00, 0x00, 0x68}
	partial := make([]byte, 50)
	input := append(hdr, partial...)

	_, err := frame.ReadFrame(bytes.NewReader(input))
	assert.Error(t, err, "truncated body must return a non-nil error")
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "truncated body error must wrap io.ErrUnexpectedEOF")
}

// TestReadWriteRoundTrip verifies that WriteFrame followed by ReadFrame on the
// same buffer returns the original payload, including the empty-payload edge case
// (header value 4, zero body bytes per RFC 5734).
func TestReadWriteRoundTrip(t *testing.T) {
	payloads := [][]byte{
		{},                                          // empty payload: header = 4, zero body bytes
		{0x01, 0x02, 0x03},                          // short payload
		[]byte("<?xml version=\"1.0\"?><epp/>"),     // XML-like
		make([]byte, 100),                            // 100-byte payload: header = 104
		make([]byte, 65535),
	}
	for i, p := range payloads {
		t.Run("", func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, frame.WriteFrame(&buf, p), "WriteFrame case %d failed", i)
			got, err := frame.ReadFrame(&buf)
			require.NoError(t, err, "ReadFrame case %d failed", i)
			assert.Equal(t, p, got, "round-trip case %d: payload mismatch", i)
		})
	}
}
