//go:build unit

package epp

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFrameHeader verifies the RFC 5734 invariant:
// totalLength = 4 (header) + len(payload).
func TestWriteFrameHeader(t *testing.T) {
	tests := []struct {
		name       string
		payload    []byte
		wantHdrVal uint32
	}{
		{"zero payload", []byte{}, 4},
		{"100-byte payload", make([]byte, 100), 104},
		{"single byte", []byte{0xAB}, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteFrame(&buf, tc.payload)
			require.NoError(t, err)
			b := buf.Bytes()
			require.GreaterOrEqual(t, len(b), 4, "buffer too short to contain header")
			got := binary.BigEndian.Uint32(b[:4])
			assert.Equal(t, tc.wantHdrVal, got,
				"EPP RFC 5734: totalLen must equal 4+len(payload)")
		})
	}
}

// TestReadFrameRoundTrip verifies ReadFrame recovers exactly what WriteFrame wrote.
func TestReadFrameRoundTrip(t *testing.T) {
	payloads := [][]byte{
		{},
		{0x01, 0x02, 0x03},
		[]byte("<?xml version=\"1.0\"?><epp/>"),
		make([]byte, 65535),
	}
	for i, p := range payloads {
		t.Run("", func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, WriteFrame(&buf, p))
			got, err := ReadFrame(&buf)
			require.NoError(t, err, "case %d", i)
			assert.Equal(t, p, got, "case %d: round-trip mismatch", i)
		})
	}
}

// TestReadFrameRejectShortLength verifies the off-by-four guard:
// length < 4 is invalid and must return an error.
func TestReadFrameRejectShortLength(t *testing.T) {
	// Header value 3 means total length < 4 — impossible valid frame
	bad := []byte{0x00, 0x00, 0x00, 0x03}
	_, err := ReadFrame(bytes.NewReader(bad))
	assert.Error(t, err, "length=3 must be rejected")
}

// TestReadFrameHeaderOnly verifies that length=4 (header only, zero body) is valid.
func TestReadFrameHeaderOnly(t *testing.T) {
	hdr := []byte{0x00, 0x00, 0x00, 0x04}
	got, err := ReadFrame(bytes.NewReader(hdr))
	require.NoError(t, err)
	assert.Equal(t, []byte{}, got)
}

// TestReadFramePartialBody verifies io.ErrUnexpectedEOF on truncated payload.
func TestReadFramePartialBody(t *testing.T) {
	// Header says 8 bytes total (4 payload), but only 2 payload bytes provided
	hdr := []byte{0x00, 0x00, 0x00, 0x08, 0xAB, 0xCD} // 4-byte hdr + 2 body bytes (should be 4)
	_, err := ReadFrame(bytes.NewReader(hdr))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}
