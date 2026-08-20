// Package fault provides configurable fault simulation for the standalone EPP
// and RRI mock servers (cmd/mock-epp-server, cmd/mock-rri-server).
//
// It is imported only by cmd/ packages — never by pkg/ test infrastructure.
// The in-process MockEPPServer and MockRRIServer are out of scope (D-09).
//
// Typical usage per connection:
//
//	engine := fault.NewFaultEngine(profile)
//	if action, _ := engine.OnConnect(conn); action == fault.ActionDisconnect { return }
//	// ... send greeting ...
//	if engine.AfterGreeting() == fault.ActionDisconnect { return }
//	if action, _ := engine.OnLoginAttempt(conn); action != fault.ActionAllow { /* handle */ }
//	action, delay := engine.OnOperation(rawFrame)
//	body, writeAction := engine.ApplyResponseFault(conn, respBody)
package fault
