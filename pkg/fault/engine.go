package fault

import (
	"encoding/binary"
	"net"
	"strings"
	"time"
)

// FaultEngine holds per-connection mutable state for fault simulation.
// Create one per handleConn goroutine via NewFaultEngine. Not safe for concurrent use.
type FaultEngine struct {
	profile       FaultProfile
	loginAttempts int
	opCount       int
	faultFired    bool
}

// NewFaultEngine returns a per-connection FaultEngine.
// profile.ParseDurations() must have been called before this.
func NewFaultEngine(profile FaultProfile) *FaultEngine {
	return &FaultEngine{profile: profile}
}

// OnConnect applies connect_delay and handles pre-greeting disconnect.
// Sleeps for connect_delay if configured, then returns ActionDisconnect if
// DisconnectAt is "pre-greeting", otherwise ActionAllow.
// The conn parameter is accepted for interface consistency but is not written to.
func (e *FaultEngine) OnConnect(_ net.Conn) (FaultAction, error) {
	if e.profile.connectDelay > 0 {
		time.Sleep(e.profile.connectDelay)
	}
	if e.profile.DisconnectAt == "pre-greeting" {
		return ActionDisconnect, nil
	}
	return ActionAllow, nil
}

// AfterGreeting returns ActionDisconnect if DisconnectAt is "post-greeting",
// otherwise ActionAllow. Call this immediately after the greeting frame is sent.
func (e *FaultEngine) AfterGreeting() FaultAction {
	if e.profile.DisconnectAt == "post-greeting" {
		return ActionDisconnect
	}
	return ActionAllow
}

// OnLoginAttempt applies login_mode logic and increments the internal login counter.
// Returns ActionAllow, ActionDeny, ActionHang, or ActionDisconnect depending on the
// configured mode and how many attempts have been made (for flap mode).
// The conn parameter is accepted for interface consistency but is not written to.
func (e *FaultEngine) OnLoginAttempt(_ net.Conn) (FaultAction, error) {
	e.loginAttempts++
	switch e.profile.LoginMode {
	case "always":
		return ActionDeny, nil
	case "flap":
		if e.loginAttempts <= e.profile.LoginFailCount {
			return ActionDeny, nil
		}
		return ActionAllow, nil
	case "hang":
		return ActionHang, nil
	case "disconnect":
		return ActionDisconnect, nil
	default:
		return ActionAllow, nil
	}
}

// OnOperation increments the operation counter and evaluates disconnect-after-N,
// per-op rules, and global mismatch. Returns the FaultAction and the total delay
// the caller should sleep before writing the response (global + per-op stacked per D-06).
// The caller is responsible for sleeping; OnOperation does not sleep.
func (e *FaultEngine) OnOperation(opType string) (FaultAction, time.Duration) {
	e.opCount++

	// D-04: disconnect after N successful operations. opCount > DisconnectAfter means
	// ops 1..N succeed and op N+1 triggers disconnect.
	if e.profile.DisconnectAfter > 0 && e.opCount > e.profile.DisconnectAfter {
		return ActionDisconnect, 0
	}

	totalDelay := e.profile.responseDelay

	// D-02: per-op rules — first matching rule wins.
	for _, rule := range e.profile.PerOp {
		if rule.Match == "" || !strings.Contains(opType, rule.Match) {
			continue
		}
		if rule.Disconnect {
			return ActionDisconnect, 0
		}
		totalDelay += rule.delay
		if rule.Mismatch {
			return ActionMismatch, totalDelay
		}
		// Rule matched but only added delay (and possibly ResultCode); stop scanning.
		return ActionAllow, totalDelay
	}

	// D-08: global mismatch field — applied when no per-op rule matched.
	if e.profile.FaultMismatch != "" && strings.Contains(opType, e.profile.FaultMismatch) {
		return ActionMismatch, totalDelay
	}

	return ActionAllow, totalDelay
}

// ApplyResponseFault optionally corrupts the response before the caller writes it.
// For "length_overflow": writes a deliberately overclaimed header + body directly to conn
// and returns (nil, ActionDisconnect) — the caller must return from handleConn.
// For "invalid_xml" and "garbage": returns the corrupted body with ActionAllow so the
// caller can use frame.WriteFrame as normal.
// Only fires once per connection (faultFired guard per D-07); subsequent calls pass through.
// Mismatch is NOT handled here — it is signalled by OnOperation returning ActionMismatch.
func (e *FaultEngine) ApplyResponseFault(conn net.Conn, responseBody []byte) ([]byte, FaultAction) {
	if e.faultFired || e.profile.MalformedFrame == "" {
		return responseBody, ActionAllow
	}

	switch e.profile.MalformedFrame {
	case "length_overflow":
		// Write a header claiming len(body)+4+100 total bytes but send only len(body)+4
		// total. The client's ReadFrame will call io.ReadFull for len(body)+100 bytes
		// and block waiting for the 100 bytes that never arrive.
		e.faultFired = true
		claimedTotal := uint32(len(responseBody) + 4 + 100)
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], claimedTotal)
		if _, err := conn.Write(hdr[:]); err != nil {
			return nil, ActionDisconnect
		}
		if _, err := conn.Write(responseBody); err != nil {
			return nil, ActionDisconnect
		}
		return nil, ActionDisconnect

	case "invalid_xml":
		e.faultFired = true
		return []byte(`<?xml version="1.0"?><not valid xml<<<`), ActionAllow

	case "garbage":
		e.faultFired = true
		return []byte{0xFF, 0xFE, 0x00, 0x01, 0xDE, 0xAD, 0xBE, 0xEF}, ActionAllow

	default:
		return responseBody, ActionAllow
	}
}
