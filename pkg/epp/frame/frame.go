package frame

import (
	"encoding/binary"
	"fmt"
	"io"
)

const eppHeaderLen = 4

// ReadFrame reads one EPP frame from r.
//
// EPP RFC 5734: the 4-byte big-endian header encodes the TOTAL frame length,
// including the 4 header bytes themselves. Body length = totalLen - 4.
// A totalLen < 4 is invalid (the header cannot reference fewer bytes than itself).
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [eppHeaderLen]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("epp: read frame header: %w", err)
	}
	total := binary.BigEndian.Uint32(hdr[:])
	if total < eppHeaderLen {
		return nil, fmt.Errorf("epp: invalid frame length %d (must be >= %d)", total, eppHeaderLen)
	}
	body := make([]byte, total-eppHeaderLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("epp: read frame body: %w", err)
	}
	return body, nil
}

// WriteFrame writes data as one EPP frame to w.
//
// EPP RFC 5734: the 4-byte big-endian header value equals len(data) + 4
// (total = header + payload). This off-by-four is mandated by the RFC.
func WriteFrame(w io.Writer, data []byte) error {
	total := uint32(len(data) + eppHeaderLen)
	var hdr [eppHeaderLen]byte
	binary.BigEndian.PutUint32(hdr[:], total)
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("epp: write frame header: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("epp: write frame body: %w", err)
	}
	return nil
}
