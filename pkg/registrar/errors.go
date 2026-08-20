package registrar

import "fmt"

// EPPError carries an EPP result code and associated message from the server.
// Callers use errors.As to extract and inspect the code:
//
//	var eppErr *EPPError
//	if errors.As(err, &eppErr) && eppErr.Code == 2302 {
//	    // object already exists — RFC 5730 §3
//	}
//
// EPP result code ranges (RFC 5730 §3):
//
//	1xxx — success
//	2xxx — error
//	2302 — object exists
//	2303 — object does not exist
//	2201 — authorization error
//	2306 — parameter value policy error
type EPPError struct {
	// Code is the numeric EPP result code from RFC 5730 §3 (e.g., 1000, 2302).
	Code int
	// Message is the human-readable result message from the server.
	Message string
	// Reason is the optional reason text from the server's <extValue> element.
	Reason string
	// Command is the EPP command that triggered this error (e.g., "domain:create").
	Command string
}

// Error implements the error interface.
func (e *EPPError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("EPP %d: %s (%s)", e.Code, e.Message, e.Reason)
	}

	return fmt.Sprintf("EPP %d: %s", e.Code, e.Message)
}

// Is supports errors.Is matching on Code only, enabling sentinel-style matching:
//
//	if errors.Is(err, &EPPError{Code: 2302}) { ... }
//
// A zero Code in the target matches any EPPError.
func (e *EPPError) Is(target error) bool {
	t, ok := target.(*EPPError)
	if !ok {
		return false
	}
	// Zero Code in target means "match any EPPError".
	return t.Code == 0 || t.Code == e.Code
}
