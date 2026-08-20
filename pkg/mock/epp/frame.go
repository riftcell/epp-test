package epp

import (
	"io"

	"github.com/riftcell/epp-test/pkg/epp/frame"
)

// ReadFrame reads one EPP frame. See pkg/epp/frame for the RFC 5734 framing rule.
func ReadFrame(r io.Reader) ([]byte, error) { return frame.ReadFrame(r) } //nolint:wrapcheck // thin wrapper — caller context is at the mock server level

// WriteFrame writes data as one EPP frame. See pkg/epp/frame for the RFC 5734 framing rule.
func WriteFrame(w io.Writer, data []byte) error { return frame.WriteFrame(w, data) } //nolint:wrapcheck // thin wrapper — caller context is at the mock server level
