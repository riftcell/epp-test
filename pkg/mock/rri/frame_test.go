//go:build unit

package rri

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteRRIFrameHeaderPayloadOnly verifies the RRI framing invariant:
// the 4-byte header encodes payload length ONLY (not +4 like EPP RFC 5734).
func TestWriteRRIFrameHeaderPayloadOnly(t *testing.T) {
	tests := []struct {
		name       string
		payload    []byte
		wantHdrVal uint32
	}{
		// RRI: header = len(payload), NOT len(payload)+4
		{"1-byte payload", []byte{0xAB}, 1},
		{"100-byte payload", make([]byte, 100), 100},
		{"10-byte payload", make([]byte, 10), 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteRRIFrame(&buf, tc.payload)
			require.NoError(t, err)
			b := buf.Bytes()
			require.GreaterOrEqual(t, len(b), 4)
			got := binary.BigEndian.Uint32(b[:4])
			assert.Equal(t, tc.wantHdrVal, got,
				"RRI framing: header must equal len(payload), not len(payload)+4")
		})
	}
}

// TestRRIFrameRoundTrip verifies WriteRRIFrame + ReadRRIFrame recover original bytes.
func TestRRIFrameRoundTrip(t *testing.T) {
	payloads := [][]byte{
		{0x01},
		[]byte("version: 5.0\naction: LOGIN\nuser: TEST\npassword: abc\n"),
		make([]byte, 1000),
	}
	for i, p := range payloads {
		t.Run("", func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, WriteRRIFrame(&buf, p))
			got, err := ReadRRIFrame(&buf)
			require.NoError(t, err, "case %d", i)
			assert.Equal(t, p, got, "case %d: round-trip mismatch", i)
		})
	}
}

// TestReadRRIFrameRejectEmpty verifies that a frame with header=0 is rejected.
func TestReadRRIFrameRejectEmpty(t *testing.T) {
	hdr := []byte{0x00, 0x00, 0x00, 0x00}
	_, err := ReadRRIFrame(bytes.NewReader(hdr))
	assert.Error(t, err, "header=0 (empty message) must be rejected")
}

// TestReadRRIFrameRejectOversized verifies that messages > 65536 bytes are rejected.
func TestReadRRIFrameRejectOversized(t *testing.T) {
	// 65537 = 0x00010001
	hdr := []byte{0x00, 0x01, 0x00, 0x01}
	_, err := ReadRRIFrame(bytes.NewReader(hdr))
	assert.Error(t, err, "payload > 65536 must be rejected")
}

// TestReadRRIFramePartialBody verifies io.ErrUnexpectedEOF on truncated payload.
func TestReadRRIFramePartialBody(t *testing.T) {
	// Header says 10 bytes payload, only 5 provided
	hdr := []byte{0x00, 0x00, 0x00, 0x0A, 0x01, 0x02, 0x03, 0x04, 0x05}
	_, err := ReadRRIFrame(bytes.NewReader(hdr))
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestRRIFramingDiffersFromEPP is a documentation test confirming RRI != EPP framing.
// For a 100-byte payload: RRI header = 100, EPP header = 104.
func TestRRIFramingDiffersFromEPP(t *testing.T) {
	payload := make([]byte, 100)

	var rriBuf bytes.Buffer
	require.NoError(t, WriteRRIFrame(&rriBuf, payload))
	rriHdr := binary.BigEndian.Uint32(rriBuf.Bytes()[:4])

	assert.Equal(t, uint32(100), rriHdr, "RRI: 100-byte payload → header=100")
	// If EPP frame.go is importable, EPP would give 104. This test documents the difference.
	assert.NotEqual(t, uint32(104), rriHdr, "RRI header must NOT equal EPP header value for same payload")
}
