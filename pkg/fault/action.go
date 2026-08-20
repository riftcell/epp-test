package fault

// FaultAction is the outcome returned by FaultEngine query methods.
// The caller (handleConn) switches on the action to decide what to do next.
type FaultAction string

const (
	// ActionAllow means proceed normally.
	ActionAllow FaultAction = "allow"
	// ActionDeny means send a protocol-level failure response
	// (EPP 2200 / RRI "RESULT: failed") and continue the session loop.
	ActionDeny FaultAction = "deny"
	// ActionHang means read the frame but never send a response.
	// The client's read deadline governs when it gives up.
	ActionHang FaultAction = "hang"
	// ActionDisconnect means close the connection immediately.
	// Callers must return from handleConn; the deferred conn.Close() fires.
	ActionDisconnect FaultAction = "disconnect"
	// ActionMismatch means respond with a wrong resource name (EPP only, D-08).
	// The caller in cmd/mock-epp-server builds the appropriate mismatch response.
	ActionMismatch FaultAction = "mismatch"
)
