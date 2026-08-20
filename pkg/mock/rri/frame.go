package rri

import (
	"encoding/binary"
	"fmt"
	"io"
)

const rriHeaderLen = 4

// ReadRRIFrame reads one RRI frame from r.
//
// DENIC RRI wire format (verified from go-rriclient@v1.26.0/pkg/rri/common.go):
// the 4-byte big-endian header encodes the payload length ONLY — it does NOT
// include the 4 header bytes themselves. Body length = exactly the header value.
//
// This is the INVERSE of EPP RFC 5734 framing (which includes the header in the count).
// Do not reuse ReadFrame from pkg/mock/epp — the semantics differ by exactly 4 bytes.
func ReadRRIFrame(r io.Reader) ([]byte, error) {
	var hdr [rriHeaderLen]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("rri: read frame header: %w", err)
	}
	payloadLen := binary.BigEndian.Uint32(hdr[:])
	if payloadLen == 0 {
		return nil, fmt.Errorf("rri: empty message (header=0)")
	}
	// Guard against absurdly large messages that could exhaust memory.
	if payloadLen > 65536 {
		return nil, fmt.Errorf("rri: message too large: %d bytes (max 65536)", payloadLen)
	}
	body := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("rri: read frame body: %w", err)
	}
	return body, nil
}

// WriteRRIFrame writes data as one RRI frame to w.
//
// DENIC RRI wire format: the 4-byte big-endian header value equals len(data)
// (payload length only — no +4 offset). This differs from EPP RFC 5734.
func WriteRRIFrame(w io.Writer, data []byte) error {
	var hdr [rriHeaderLen]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("rri: write frame header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("rri: write frame body: %w", err)
	}
	return nil
}
