// Command mock-rri-server is a standalone DENIC RRI TCP mock server for manual integration testing.
//
// It listens on 127.0.0.1:7701 by default, accepts any TCP client with no credential
// validation, and returns "RESULT: success" with an incrementing STID for LOGIN and all
// queries. The connection is closed after LOGOUT, mirroring the real DENIC server.
// Ctrl+C / SIGTERM triggers a graceful shutdown.
//
// Usage:
//
//	go run ./cmd/mock-rri-server
//	go run ./cmd/mock-rri-server -addr 0.0.0.0:7701
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/riftcell/epp-test/pkg/fault"
	"github.com/riftcell/epp-test/pkg/mock/rri"
)

var stidCounter atomic.Int64

// loadFaultProfile loads a YAML fault profile and applies CLI flag overrides.
// Only 6 RRI-applicable flags are handled; malformed_frame and fault_mismatch
// are EPP-only (D-07, D-08) and not exposed as CLI flags on this server.
// Uses flag.Visit to detect explicitly-set flags only (RESEARCH.md Pitfall 3).
func loadFaultProfile(
	profilePath, connectDelay, loginMode string,
	loginFailCount int,
	responseDelay string,
	disconnectAfter int,
) (fault.FaultProfile, error) {
	profile, err := fault.LoadProfile(profilePath)
	if err != nil {
		return fault.FaultProfile{}, fmt.Errorf("load fault profile: %w", err)
	}

	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "connect-delay":
			profile.ConnectDelay = connectDelay
		case "login-mode":
			profile.LoginMode = loginMode
		case "login-fail-count":
			profile.LoginFailCount = loginFailCount
		case "response-delay":
			profile.ResponseDelay = responseDelay
		case "disconnect-after":
			profile.DisconnectAfter = disconnectAfter
		}
	})

	return profile, profile.ParseDurations() //nolint:wrapcheck // pkg/fault error is self-describing; wrapping adds no context
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7701", "TCP address to listen on (host:port)")
	faultProfile := flag.String("fault-profile", "", "path to YAML fault profile (flags override YAML values)")
	connectDelay := flag.String("connect-delay", "", "sleep before first frame read (e.g. 2s, 500ms) [D-05]")
	loginMode := flag.String("login-mode", "", "login fault: always|flap|hang|disconnect [D-03]")
	loginFailCount := flag.Int("login-fail-count", 0, "login failures before allow in flap mode [D-03]")
	responseDelay := flag.String("response-delay", "", "global response delay (e.g. 100ms) [D-06]")
	disconnectAfter := flag.Int("disconnect-after", 0, "close after N operations, 0=disabled [D-04]")
	flag.Parse()

	profile, err := loadFaultProfile(
		*faultProfile,
		*connectDelay, *loginMode, *loginFailCount,
		*responseDelay, *disconnectAfter,
	)
	if err != nil {
		log.Fatalf("mock-rri-server: %v", err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("mock-rri-server: listen %s: %v", *addr, err)
	}
	defer ln.Close() //nolint:errcheck // best-effort close in defer; server shutdown, see CONVENTIONS.md §3

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("mock-rri-server listening on %s", *addr)

	go func() {
		<-ctx.Done()
		log.Printf("mock-rri-server: shutting down")
		ln.Close() //nolint:errcheck // best-effort close; unblocks Accept so the accept loop can exit
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("mock-rri-server: accept error: %v", err)
			continue
		}
		go handleConn(conn, profile)
	}
}

func handleConn(conn net.Conn, profile fault.FaultProfile) {
	defer conn.Close() //nolint:errcheck // best-effort close in defer; connection teardown on exit, see CONVENTIONS.md §3

	engine := fault.NewFaultEngine(profile)

	// Step 1: Connect delay + pre-greeting disconnect (D-05, D-04).
	// For RRI, "pre-greeting" means disconnect before the first frame read.
	if action, _ := engine.OnConnect(conn); action == fault.ActionDisconnect { //nolint:errcheck // OnConnect never returns a non-nil error; conn parameter is unused in current impl
		return
	}

	if err := conn.SetDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		log.Printf("mock-rri-server: SetDeadline: %v", err)
		return
	}

	// Step 2: "Post-greeting" disconnect for RRI means disconnect after accepting
	// the connection, before reading any frame (RRI has no greeting frame, D-04).
	if engine.AfterGreeting() == fault.ActionDisconnect {
		return
	}

	loggedIn := false

	for {
		frameBytes, err := rri.ReadRRIFrame(conn)
		if err != nil {
			return
		}

		fields := rri.ParseKV(string(frameBytes))
		// ParseKV lowercases all keys; look up "action" in lowercase.
		action := strings.ToUpper(firstAction(fields))
		log.Printf("rri <- %s", action)

		switch action {
		case "LOGIN":
			loginAction, _ := engine.OnLoginAttempt(conn) //nolint:errcheck // OnLoginAttempt never returns a non-nil error; conn parameter is unused in current impl
			switch loginAction {
			case fault.ActionHang:
				// Do not respond; block on next ReadRRIFrame.
				continue
			case fault.ActionDisconnect:
				return
			case fault.ActionDeny:
				n := stidCounter.Add(1)
				failResp := []byte(fmt.Sprintf("RESULT: failed\nSTID: %d\n", n))
				if err := rri.WriteRRIFrame(conn, failResp); err != nil {
					log.Printf("mock-rri-server: write LOGIN-deny: %v", err)
					return
				}
				continue
			default: // ActionAllow
				loggedIn = true
				if err := rri.WriteRRIFrame(conn, successResp()); err != nil {
					log.Printf("mock-rri-server: write LOGIN response: %v", err)
					return
				}
			}

		case "LOGOUT":
			if err := rri.WriteRRIFrame(conn, successResp()); err != nil {
				log.Printf("mock-rri-server: write LOGOUT response: %v", err)
			}
			// Real DENIC closes after LOGOUT; go-rriclient waits for EOF.
			return

		default:
			_ = loggedIn // state tracked but not enforced — any query succeeds

			// Apply fault engine for non-LOGIN, non-LOGOUT ops (D-04, D-06).
			// opType is the full raw KV frame string for per-op substring matching (D-02).
			opAction, delay := engine.OnOperation(string(frameBytes))
			if delay > 0 {
				time.Sleep(delay)
			}
			if opAction == fault.ActionDisconnect {
				return
			}
			// ActionMismatch: not applicable to RRI (D-08). Treat as ActionAllow.
			// ApplyResponseFault: not called (D-07 malformed frame is EPP-only).

			if err := rri.WriteRRIFrame(conn, successResp()); err != nil {
				log.Printf("mock-rri-server: write response: %v", err)
				return
			}
		}
	}
}

// successResp builds a KV-format RRI success response with a unique STID.
// Format matches pkg/mock/rri buildSuccessResponse: RESULT and STID keys.
func successResp() []byte {
	n := stidCounter.Add(1)
	return []byte(fmt.Sprintf("RESULT: success\nSTID: %d\n", n))
}

// firstAction returns the first value for the "action" key from a ParseKV result map.
// ParseKV lowercases all keys, so the lookup key must be lowercase.
func firstAction(fields map[string][]string) string {
	vals := fields["action"]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
