package epp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	nbioxml "github.com/nbio/xml"

	eppframe "github.com/riftcell/epp-test/pkg/epp/frame"
	eppxml "github.com/riftcell/epp-test/pkg/epp/xml"
	"github.com/riftcell/epp-test/pkg/registrar"
)

// buildTLSConfig constructs a tls.Config from the adapter's RegistrarConfig.
// If WithTLSConfig was used, the injected config is returned directly.
// Otherwise: MinVersion=TLS1.2, ServerName=cfg.Host, RootCAs from CAFile (or system
// pool), and optional client certificate from CertFile/KeyFile.
func (a *GenericEPPAdapter) buildTLSConfig() (*tls.Config, error) {
	// If a TLS config was injected via WithTLSConfig (test path), use it directly.
	if a.tlsConfig != nil {
		return a.tlsConfig, nil
	}

	tlsCfg := &tls.Config{
		ServerName: a.cfg.Host,
		MinVersion: tls.VersionTLS12,
	}

	// Load server CA certificate for verification.
	// nil RootCAs means the system pool, which is fine for real EPP servers.
	if a.cfg.CAFile != "" {
		caPEM, err := os.ReadFile(a.cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("epp: read CA file %q: %w", a.cfg.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("epp: no valid certificate in CA file %q", a.cfg.CAFile)
		}
		tlsCfg.RootCAs = pool
	}

	// Load client certificate for mutual TLS (mTLS).
	if a.cfg.CertFile != "" && a.cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(a.cfg.CertFile, a.cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("epp: load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// dial establishes a TLS connection to the EPP server, reads the server greeting,
// and sends the login command. This is called both by Login (initial connect) and by
// reconnect (after connection loss). CRITICAL (Pitfall 4): the greeting is read on
// EVERY dial because the server sends it unsolicited on every new connection.
func (a *GenericEPPAdapter) dial(ctx context.Context) error {
	tlsCfg, err := a.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("epp: build TLS config: %w", err)
	}

	addr := net.JoinHostPort(a.cfg.Host, fmt.Sprintf("%d", a.cfg.Port))
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    tlsCfg,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("epp: dial %s: %w", addr, err)
	}
	a.conn = conn

	// Greeting is always the first frame from the server; must be read before login.
	if err := a.readGreeting(); err != nil {
		// Best-effort close: the connection is unusable, so a close error here
		// is not actionable (CONVENTIONS.md §3: discarding a close error outside
		// a defer chain requires this justification).
		_ = a.conn.Close() //nolint:errcheck // best-effort close; connection unusable after greeting failure, see CONVENTIONS.md §3
		a.conn = nil
		return fmt.Errorf("epp: read greeting: %w", err)
	}

	if err := a.sendLogin(ctx); err != nil {
		// Best-effort close: the login failed, so the connection is unusable and
		// a close error here is not actionable (CONVENTIONS.md §3).
		_ = a.conn.Close() //nolint:errcheck // best-effort close; connection unusable after login failure, see CONVENTIONS.md §3
		a.conn = nil
		return fmt.Errorf("epp: login: %w", err)
	}

	return nil
}

// readGreeting reads and parses the EPP server greeting frame. It stores the
// advertised extension URIs and intersects them with cfg.Extensions. Extensions
// requested in cfg but not advertised by the server are logged as warnings and
// dropped from the login request (Pitfall 5: svcExtension mismatch causes code 2306).
func (a *GenericEPPAdapter) readGreeting() error {
	raw, err := eppframe.ReadFrame(a.conn)
	if err != nil {
		return fmt.Errorf("read frame: %w", err)
	}

	// The greeting is delivered as a full EPP envelope: <epp:epp><epp:greeting>...</epp:greeting></epp:epp>.
	// Unmarshal into EPPEnvelope and extract the Greeting field.
	var env eppxml.EPPEnvelope
	if err := nbioxml.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("unmarshal greeting envelope: %w", err)
	}
	if env.Greeting == nil {
		return fmt.Errorf("epp: server sent frame without <epp:greeting>")
	}
	g := env.Greeting

	// Build a set of extension URIs advertised by the server.
	advertised := make(map[string]bool)
	if g.SvcMenu.SvcExtension != nil {
		for _, uri := range g.SvcMenu.SvcExtension.ExtURI {
			advertised[uri] = true
		}
	}

	// Compute the intersection: only request extensions that the server advertises.
	// Log a warning for any configured extension that the server did not advertise.
	// Proceeding with the intersection (not fail-hard) per Open Question 3 recommendation:
	// failing would break OT&E when extension namespaces are discovered iteratively.
	filtered := make([]string, 0, len(a.cfg.Extensions))
	for _, uri := range a.cfg.Extensions {
		if advertised[uri] {
			filtered = append(filtered, uri)
		} else {
			log.Printf("epp[%s]: warning: configured extension %q not advertised by server; skipping", a.name, uri)
		}
	}
	a.advertisedExt = filtered

	return nil
}

// sendLogin marshals and sends an epp:login command, then reads and validates the response.
// The login uses the filtered extension list from readGreeting.
// Must be called with a.mu already held (via Login or reconnect).
func (a *GenericEPPAdapter) sendLogin(_ context.Context) error {
	// Standard EPP object URIs that all compliant servers support.
	objURIs := []string{
		"urn:ietf:params:xml:ns:domain-1.0",
		"urn:ietf:params:xml:ns:contact-1.0",
		"urn:ietf:params:xml:ns:host-1.0",
	}

	login := &eppxml.Login{
		ClID: a.cfg.Username,
		PW:   a.cfg.Password,
		Options: eppxml.LoginOptions{
			Version: "1.0",
			Lang:    "en",
		},
		Svcs: eppxml.LoginSvcs{
			ObjURI: objURIs,
		},
	}

	// Include the filtered extension list if non-empty.
	if len(a.advertisedExt) > 0 {
		login.Svcs.SvcExtension = &eppxml.SvcExtension{ExtURI: a.advertisedExt}
	}

	// clTRID is generated without locking because sendLogin is always called while
	// the caller already holds a.mu (Login and reconnect both hold the lock).
	a.clTRID++
	clTRID := fmt.Sprintf("gsd-%s-%d", a.name, a.clTRID)

	env := eppxml.EPPEnvelope{
		Command: &eppxml.EPPCommand{
			Login:  login,
			ClTRID: clTRID,
		},
	}

	payload, err := nbioxml.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal login: %w", err)
	}

	if err := eppframe.WriteFrame(a.conn, payload); err != nil {
		return fmt.Errorf("write login frame: %w", err)
	}

	resp, err := eppframe.ReadFrame(a.conn)
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}

	var envResp eppxml.EPPEnvelope
	if err := nbioxml.Unmarshal(resp, &envResp); err != nil {
		return fmt.Errorf("unmarshal login response: %w", err)
	}

	if envResp.Response == nil {
		return fmt.Errorf("epp: login response missing <epp:response>")
	}

	return a.resultToError(envResp.Response.Result, "epp:login")
}

// reconnect performs a best-effort close of the current connection, waits 500ms,
// then re-dials (which re-reads the greeting and re-logs in). This is the single
// retry path used by withConn when a mid-operation network error occurs (D-04).
func (a *GenericEPPAdapter) reconnect(ctx context.Context) error {
	if a.conn != nil {
		// Best-effort close before replacing the connection; the old conn is being
		// discarded so a close error here is not actionable (CONVENTIONS.md §3:
		// discarding a close error outside a defer chain requires this justification).
		_ = a.conn.Close() //nolint:errcheck // best-effort close; old connection being replaced, see CONVENTIONS.md §3
		a.conn = nil
	}

	// 500ms backoff before reconnect (D-04, RESEARCH Pattern 5).
	time.Sleep(500 * time.Millisecond)

	// dial re-reads the greeting and re-logs in (Pitfall 4: greeting on every dial).
	return a.dial(ctx)
}

// withConn acquires the adapter mutex, executes fn with the current connection, and
// performs ONE reconnect + retry on TRANSPORT failures only. The single-retry policy
// (D-04) protects against transient connection resets without masking real server-side
// errors. Protocol-level errors (*EPPError with 2xxx codes) are not transport failures
// and do NOT trigger a reconnect — only network/IO errors do.
// The mutex serializes all EPP operations on the shared TLS connection.
func (a *GenericEPPAdapter) withConn(ctx context.Context, fn func(conn net.Conn) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := fn(a.conn); err != nil {
		// Do not reconnect for protocol-level errors (EPPError = server-side error codes).
		// Only reconnect for transport errors (EOF, connection reset, IO timeout, etc.).
		var eppErr *registrar.EPPError
		if errors.As(err, &eppErr) {
			return err
		}
		// One reconnect attempt with 500ms backoff before retrying (D-04).
		if reconnErr := a.reconnect(ctx); reconnErr != nil {
			return fmt.Errorf("epp: operation failed and reconnect failed: %w", reconnErr)
		}
		return fn(a.conn)
	}
	return nil
}

// resultToError maps an EPP result code to an error. Codes below 2000 indicate
// success and return nil. Codes 2xxx indicate server-side errors and return
// *registrar.EPPError so callers can use errors.As to inspect the code.
func (a *GenericEPPAdapter) resultToError(r eppxml.Result, command string) error {
	if r.Code < 2000 {
		return nil
	}
	return &registrar.EPPError{
		Code:    r.Code,
		Message: r.Msg,
		Reason:  r.Reason,
		Command: command,
	}
}
