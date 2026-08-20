// Command mock-epp-server is a standalone EPP TLS mock server for manual integration testing.
//
// It listens on 127.0.0.1:7700 by default, accepts any TLS client (no client-cert
// requirement), sends an EPP greeting on connect, and responds to all standard EPP
// commands with always-success 1000 responses. domain:check returns all names as
// available. Ctrl+C / SIGTERM triggers a graceful shutdown.
//
// Usage:
//
//	go run ./cmd/mock-epp-server
//	go run ./cmd/mock-epp-server -addr 0.0.0.0:7700
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/riftcell/epp-test/pkg/epp/frame"
	"github.com/riftcell/epp-test/pkg/fault"
)

var svTRIDCounter atomic.Int64

// loadFaultProfile loads a YAML fault profile from profilePath (if non-empty),
// then applies any CLI flags that were explicitly set on the command line.
//
// CRITICAL: Use flag.Visit (not flag.VisitAll). flag.Visit calls the function only
// for flags that were explicitly provided on the command line. flag.VisitAll also
// visits flags with default values, which would incorrectly override YAML settings
// with empty strings when the flag was not set (RESEARCH.md Pitfall 3).
func loadFaultProfile(
	profilePath,
	connectDelay, loginMode string,
	loginFailCount int,
	responseDelay string,
	disconnectAfter int,
	malformedFrame, faultMismatch string,
) (fault.FaultProfile, error) {
	profile, err := fault.LoadProfile(profilePath)
	if err != nil {
		return fault.FaultProfile{}, fmt.Errorf("load fault profile: %w", err)
	}

	// Override YAML with any CLI flags that were explicitly set.
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
		case "malformed-frame":
			profile.MalformedFrame = malformedFrame
		case "fault-mismatch":
			profile.FaultMismatch = faultMismatch
		}
	})

	// ParseDurations is called here (after overrides) not in LoadProfile.
	// This is the single call that resolves all duration strings to time.Duration.
	return profile, profile.ParseDurations() //nolint:wrapcheck // pkg/fault error is self-describing; wrapping adds no context
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7700", "TCP address to listen on (host:port)")
	faultProfile := flag.String("fault-profile", "", "path to YAML fault profile (flags override YAML values)")
	connectDelay := flag.String("connect-delay", "", "sleep before TLS greeting (e.g. 2s, 500ms) [D-05]")
	loginMode := flag.String("login-mode", "", "login fault: always|flap|hang|disconnect [D-03]")
	loginFailCount := flag.Int("login-fail-count", 0, "login failures before allow in flap mode [D-03]")
	responseDelay := flag.String("response-delay", "", "global response delay (e.g. 100ms) [D-06]")
	disconnectAfter := flag.Int("disconnect-after", 0, "close after N operations, 0=disabled [D-04]")
	malformedFrame := flag.String("malformed-frame", "", "length_overflow|invalid_xml|garbage [D-07]")
	faultMismatch := flag.String("fault-mismatch", "", "op substring for mismatch response (e.g. domain:create) [D-08]")
	flag.Parse()

	profile, err := loadFaultProfile(
		*faultProfile,
		*connectDelay, *loginMode, *loginFailCount,
		*responseDelay, *disconnectAfter,
		*malformedFrame, *faultMismatch,
	)
	if err != nil {
		log.Fatalf("mock-epp-server: %v", err)
	}

	cert, err := genSelfSignedCert()
	if err != nil {
		log.Fatalf("mock-epp-server: generate cert: %v", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		// No ClientAuth — any EPP client can connect without credentials.
	}

	ln, err := tls.Listen("tcp", *addr, tlsCfg)
	if err != nil {
		log.Fatalf("mock-epp-server: listen %s: %v", *addr, err)
	}
	defer ln.Close() //nolint:errcheck // best-effort close in defer; server shutdown, see CONVENTIONS.md §3

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("mock-epp-server listening on %s (TLS, no client auth)", *addr)

	go func() {
		<-ctx.Done()
		log.Printf("mock-epp-server: shutting down")
		ln.Close() //nolint:errcheck // best-effort close; unblocks Accept so the accept loop can exit
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("mock-epp-server: accept error: %v", err)
			continue
		}
		go handleConn(conn, profile)
	}
}

// genSelfSignedCert generates a self-signed ECDSA P-256 server certificate.
// SANs include 127.0.0.1 (IP) and localhost (DNS) so that TLS clients can
// verify the cert when connecting via either address.
func genSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Mock EPP Server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		// Self-signed: IsCA allows signing its own cert directly.
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("build tls.Certificate: %w", err)
	}
	return tlsCert, nil
}

func handleConn(conn net.Conn, profile fault.FaultProfile) {
	defer conn.Close() //nolint:errcheck // best-effort close in defer; connection teardown on exit, see CONVENTIONS.md §3

	engine := fault.NewFaultEngine(profile)

	// Step 1: Connect delay + pre-greeting disconnect (D-05, D-04).
	if action, _ := engine.OnConnect(conn); action == fault.ActionDisconnect { //nolint:errcheck // OnConnect never returns a non-nil error; conn parameter is unused in current impl
		return
	}

	if err := conn.SetDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		log.Printf("mock-epp-server: SetDeadline: %v", err)
		return
	}

	// Step 2: Send EPP greeting — server always speaks first (RFC 5730 §2).
	if err := frame.WriteFrame(conn, greeting()); err != nil {
		log.Printf("mock-epp-server: write greeting: %v", err)
		return
	}

	// Step 3: Post-greeting disconnect (D-04).
	if engine.AfterGreeting() == fault.ActionDisconnect {
		return
	}

	// Dispatch table for non-login, non-logout operations.
	type handler struct {
		needle  string
		builder func(xmlStr string) []byte
	}
	handlers := []handler{
		{"<hello", func(_ string) []byte { return greeting() }},
		{"domain:check", func(s string) []byte { return domainCheckResp(s) }},
		{"domain:", func(s string) []byte { return resp1000(echoCltrid(s)) }},
		{"contact:", func(s string) []byte { return resp1000(echoCltrid(s)) }},
		{"host:", func(s string) []byte { return resp1000(echoCltrid(s)) }},
	}

	for {
		body, err := frame.ReadFrame(conn)
		if err != nil {
			return
		}
		xmlStr := string(body)

		// Login frame: fault-aware path (D-03).
		if strings.Contains(xmlStr, "<login") {
			log.Printf("epp <- <login")
			action, _ := engine.OnLoginAttempt(conn) //nolint:errcheck // OnLoginAttempt never returns a non-nil error; conn parameter is unused in current impl
			switch action {
			case fault.ActionHang:
				// Do not respond; block on next ReadFrame.
				// The client's read deadline governs when the hang ends.
				continue
			case fault.ActionDisconnect:
				return
			case fault.ActionDeny:
				resp := buildResp(2200, "Authentication error; server closing connection", echoCltrid(xmlStr))
				if err := frame.WriteFrame(conn, resp); err != nil {
					log.Printf("mock-epp-server: write login-deny: %v", err)
					return
				}
				continue
			default: // ActionAllow
				if err := frame.WriteFrame(conn, resp1000(echoCltrid(xmlStr))); err != nil {
					log.Printf("mock-epp-server: write login-ok: %v", err)
					return
				}
				continue
			}
		}

		// Logout: respond with 1500 and close (EPP server ends session per RFC 5730).
		if strings.Contains(xmlStr, "<logout") {
			log.Printf("epp <- <logout")
			if err := frame.WriteFrame(conn, resp1500(echoCltrid(xmlStr))); err != nil {
				log.Printf("mock-epp-server: write logout: %v", err)
			}
			return
		}

		// All other operations: fault engine (D-02, D-04, D-06, D-07, D-08).
		opAction, delay := engine.OnOperation(xmlStr)
		if delay > 0 {
			time.Sleep(delay)
		}
		if opAction == fault.ActionDisconnect {
			return
		}

		// Build response body.
		var respBytes []byte
		if opAction == fault.ActionMismatch {
			// Mismatch: respond with sentinel wrong resource name (D-08).
			// pkg/fault does not know EPP XML; mismatch builders live here in cmd/.
			switch {
			case strings.Contains(xmlStr, "domain:create"):
				log.Printf("epp <- domain:create (mismatch)")
				respBytes = domainMismatchCreDataResp(echoCltrid(xmlStr))
			case strings.Contains(xmlStr, "contact:create"):
				log.Printf("epp <- contact:create (mismatch)")
				respBytes = contactMismatchCreDataResp(echoCltrid(xmlStr))
			default:
				// No mismatch template for this op; fall through to normal response.
				respBytes = resp1000(echoCltrid(xmlStr))
			}
		} else {
			matched := false
			for _, h := range handlers {
				if strings.Contains(xmlStr, h.needle) {
					log.Printf("epp <- %s", h.needle)
					respBytes = h.builder(xmlStr)
					matched = true
					break
				}
			}
			if !matched {
				log.Printf("epp <- unknown command")
				respBytes = resp2000()
			}
		}

		// Apply malformed frame fault to first non-login response (D-07, EPP-only).
		// For length_overflow: ApplyResponseFault writes directly to conn and returns
		//   (nil, ActionDisconnect). Do not call frame.WriteFrame in that case.
		// For invalid_xml/garbage: returns (corruptedBody, ActionAllow). Call WriteFrame.
		// For no fault / faultFired: returns (respBytes, ActionAllow). Call WriteFrame.
		respBytes, writeAction := engine.ApplyResponseFault(conn, respBytes)
		if writeAction == fault.ActionDisconnect {
			return
		}
		if err := frame.WriteFrame(conn, respBytes); err != nil {
			log.Printf("mock-epp-server: write response: %v", err)
			return
		}
	}
}

// greeting returns a minimal but well-formed EPP greeting frame body.
func greeting() []byte {
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<greeting>`+
			`<svID>Mock EPP Server</svID>`+
			`<svDate>%s</svDate>`+
			`<svcMenu>`+
			`<version>1.0</version>`+
			`<lang>en</lang>`+
			`<objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>`+
			`<objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>`+
			`<objURI>urn:ietf:params:xml:ns:host-1.0</objURI>`+
			`</svcMenu>`+
			`</greeting>`+
			`</epp>`,
		time.Now().UTC().Format(time.RFC3339),
	))
}

// echoCltrid extracts the clTRID value from an EPP XML request.
// Returns an empty string if no clTRID is present.
func echoCltrid(xmlStr string) string {
	const openTag = "<clTRID>"
	const closeTag = "</clTRID>"
	start := strings.Index(xmlStr, openTag)
	if start == -1 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(xmlStr[start:], closeTag)
	if end == -1 {
		return ""
	}
	return xmlStr[start : start+end]
}

// resp1000 returns an EPP result 1000 (command completed successfully) frame body.
func resp1000(clTRID string) []byte {
	return buildResp(1000, "Command completed successfully", clTRID)
}

// resp1500 returns an EPP result 1500 (ending session) frame body.
func resp1500(clTRID string) []byte {
	return buildResp(1500, "Command completed successfully; ending session", clTRID)
}

// resp2000 returns an EPP result 2000 (unknown command) frame body.
func resp2000() []byte {
	return buildResp(2000, "Unknown command", "")
}

// buildResp constructs a standard EPP response envelope.
func buildResp(code int, msg, clTRID string) []byte {
	n := svTRIDCounter.Add(1)
	clTRIDElem := ""
	if clTRID != "" {
		clTRIDElem = fmt.Sprintf("<clTRID>%s</clTRID>", clTRID)
	}
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<response>`+
			`<result code="%d"><msg>%s</msg></result>`+
			`<trID>%s<svTRID>mock-%d</svTRID></trID>`+
			`</response>`+
			`</epp>`,
		code, msg, clTRIDElem, n,
	))
}

// domainMismatchCreDataResp returns a domain:creData response with sentinel name
// "mismatch-sentinel.example" instead of the requested domain (D-08).
func domainMismatchCreDataResp(clTRID string) []byte {
	n := svTRIDCounter.Add(1)
	clTRIDElem := ""
	if clTRID != "" {
		clTRIDElem = fmt.Sprintf("<clTRID>%s</clTRID>", clTRID)
	}
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<response>`+
			`<result code="1000"><msg>Command completed successfully</msg></result>`+
			`<resData>`+
			`<domain:creData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+
			`<domain:name>mismatch-sentinel.example</domain:name>`+
			`<domain:crDate>%s</domain:crDate>`+
			`<domain:exDate>%s</domain:exDate>`+
			`</domain:creData>`+
			`</resData>`+
			`<trID>%s<svTRID>mock-%d</svTRID></trID>`+
			`</response>`+
			`</epp>`,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().Add(365*24*time.Hour).UTC().Format(time.RFC3339),
		clTRIDElem, n,
	))
}

// contactMismatchCreDataResp returns a contact:creData response with sentinel ID
// "MISMATCH-C" instead of the requested contact ID (D-08).
func contactMismatchCreDataResp(clTRID string) []byte {
	n := svTRIDCounter.Add(1)
	clTRIDElem := ""
	if clTRID != "" {
		clTRIDElem = fmt.Sprintf("<clTRID>%s</clTRID>", clTRID)
	}
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<response>`+
			`<result code="1000"><msg>Command completed successfully</msg></result>`+
			`<resData>`+
			`<contact:creData xmlns:contact="urn:ietf:params:xml:ns:contact-1.0">`+
			`<contact:id>MISMATCH-C</contact:id>`+
			`<contact:crDate>%s</contact:crDate>`+
			`</contact:creData>`+
			`</resData>`+
			`<trID>%s<svTRID>mock-%d</svTRID></trID>`+
			`</response>`+
			`</epp>`,
		time.Now().UTC().Format(time.RFC3339),
		clTRIDElem, n,
	))
}

// domainCheckResp builds a domain:chkData response with all queried names marked available.
// It parses domain names from the request using a simple string scan.
func domainCheckResp(xmlStr string) []byte {
	names := extractDomainNames(xmlStr)
	if len(names) == 0 {
		names = []string{"example.com"}
	}

	var cdItems strings.Builder
	for _, name := range names {
		fmt.Fprintf(&cdItems,
			`<domain:cd><domain:name avail="1">%s</domain:name></domain:cd>`,
			name,
		)
	}

	n := svTRIDCounter.Add(1)
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<response>`+
			`<result code="1000"><msg>Command completed successfully</msg></result>`+
			`<resData>`+
			`<domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+
			`%s`+
			`</domain:chkData>`+
			`</resData>`+
			`<trID><svTRID>mock-%d</svTRID></trID>`+
			`</response>`+
			`</epp>`,
		cdItems.String(), n,
	))
}

// extractDomainNames scans xmlStr for <domain:name ...>NAME</domain:name> patterns.
// It returns all names found, without duplicates.
func extractDomainNames(xmlStr string) []string {
	const closeTag = "</domain:name>"
	var names []string
	seen := make(map[string]bool)

	rest := xmlStr
	for {
		// Find the closing tag first to locate the name.
		endIdx := strings.Index(rest, closeTag)
		if endIdx == -1 {
			break
		}
		// Find the start of the element (opening > before the name).
		segment := rest[:endIdx]
		openIdx := strings.LastIndex(segment, ">")
		if openIdx == -1 {
			rest = rest[endIdx+len(closeTag):]
			continue
		}
		name := strings.TrimSpace(segment[openIdx+1:])
		if name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
		rest = rest[endIdx+len(closeTag):]
	}
	return names
}
