package rri

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// testingT is the minimal *testing.T surface the mock server constructors
// need.
type testingT interface {
	Helper()
	Fatal(...any)
	Cleanup(func())
}

// MockRRIServer is an in-process DENIC RRI mock server for unit testing.
//
// Create one per test using NewMockRRIServer(t). The constructor calls
// t.Cleanup automatically — do not call Close manually.
//
// Register users before connecting:
//
//	srv.AddUser("DENIC-1234", "plaintextpassword")
//
// The mock stores MD5 hashes internally. The connecting client must send the
// MD5 hex digest in the password field (not plaintext), exactly as DENIC requires.
//
// Scripted response API:
//
//	srv.Expect <- []byte("RESULT: success\nSTID: abc\n")
//	frame := <-srv.Received
type MockRRIServer struct {
	// Expect is the scripted response queue for post-login commands.
	// Push []byte KV-format responses. LOGIN and pre-login rejections
	// are handled automatically and do NOT consume from Expect.
	Expect chan any

	// Received captures every frame the server reads from the client
	// (including LOGIN frames). Non-blocking send: frames are dropped
	// if the buffer is full.
	Received chan []byte

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	usersMu sync.RWMutex
	users   map[string]string // username -> md5Hex(plaintextPassword)

	mu    sync.Mutex
	delay time.Duration

	connMu sync.Mutex
	conn   net.Conn // active connection, for DropConnection

	stidCounter atomic.Int64 // monotonic counter for unique STIDs
}

// NewMockRRIServer creates a new in-process RRI mock server for the given test.
//
// The server listens on 127.0.0.1:0 (OS-assigned port) using plain TCP.
// t.Cleanup is called automatically — do not call Close manually.
//
// Use this for RRI clients that dial plain TCP (or that accept a
// TLSDialHandler-style injection point typed as a generic connection
// interface). Clients whose dial hook is typed to return a concrete
// *tls.Conn — such as wcs-externalapi's rri.Client — cannot connect to a
// plain-TCP listener; use NewMockRRIServerTLS instead.
func NewMockRRIServer(t testingT) *MockRRIServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("NewMockRRIServer: net.Listen:", err)
	}

	return newMockRRIServer(t, ln)
}

// NewMockRRIServerTLS creates a new in-process RRI mock server for the given
// test, listening over TLS with a self-signed certificate instead of plain
// TCP.
//
// t.Cleanup is called automatically — do not call Close manually. Everything
// else (Expect/Received scripting, AddUser login simulation, SetDelay) works
// identically to NewMockRRIServer; only the transport differs.
//
// Use this for RRI clients whose dial hook requires a real TLS connection,
// such as wcs-externalapi's rri.Client (its TLSDialHandler is typed to
// return a concrete *tls.Conn, so it cannot be pointed at a plain-TCP
// listener the way NewMockRRIServer provides). The generated certificate is
// self-signed and not accompanied by a CA pool: DENIC RRI clients dial with
// InsecureSkipVerify, so nothing validates the certificate chain.
func NewMockRRIServerTLS(t testingT) *MockRRIServer {
	t.Helper()

	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatal("NewMockRRIServerTLS: generateSelfSignedCert:", err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal("NewMockRRIServerTLS: tls.Listen:", err)
	}

	return newMockRRIServer(t, ln)
}

// newMockRRIServer wires up the shared accept loop and lifecycle for both
// NewMockRRIServer and NewMockRRIServerTLS — the two differ only in how ln
// was constructed.
func newMockRRIServer(t testingT, ln net.Listener) *MockRRIServer {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	srv := &MockRRIServer{
		Expect:   make(chan any, 32),
		Received: make(chan []byte, 32),
		listener: ln,
		cancel:   cancel,
		users:    make(map[string]string),
	}

	srv.wg.Add(1)
	go srv.acceptLoop(ctx)

	t.Cleanup(func() {
		cancel()
		_ = ln.Close() //nolint:errcheck // best-effort close in Cleanup; upstream error already captured, see CONVENTIONS.md §3
		srv.wg.Wait()
	})

	return srv
}

// Addr returns the server's TCP address in the form "127.0.0.1:PORT".
func (s *MockRRIServer) Addr() string {
	return s.listener.Addr().String()
}

// AddUser registers a user for login validation.
//
// plaintextPassword is hashed with MD5 before storage. The connecting client
// must send the MD5 hex digest (not plaintext) in the LOGIN password field —
// this mirrors the real DENIC server's requirement (MOCK-02).
func (s *MockRRIServer) AddUser(username, plaintextPassword string) {
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	s.users[username] = md5Hex(plaintextPassword)
}

// SetDelay configures a sleep duration applied before each scripted response.
// SetDelay(0) disables the delay. Safe to call concurrently.
func (s *MockRRIServer) SetDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = d
}

// DropConnection closes the active client connection abruptly. The next read
// or write by the client will return io.EOF or a closed-connection error.
func (s *MockRRIServer) DropConnection() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close() //nolint:errcheck // best-effort close; method intent is abrupt connection drop, see CONVENTIONS.md §3
	}
}

func (s *MockRRIServer) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.connMu.Lock()
		s.conn = conn
		s.connMu.Unlock()

		s.wg.Add(1)
		go s.handleConn(ctx, conn)
	}
}

func (s *MockRRIServer) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close() //nolint:errcheck // best-effort close in defer; connection teardown on exit, see CONVENTIONS.md §3

	// 30-second deadline prevents handler from hanging on a panicking client.
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}

	// loggedIn is per-connection state — not shared (MOCK-02).
	loggedIn := false

	for {
		frame, err := ReadRRIFrame(conn)
		if err != nil {
			return
		}

		// Capture the raw frame (non-blocking: buffer-full drops are acceptable).
		select {
		case s.Received <- frame:
		default:
		}

		fields := ParseKV(string(frame))
		action := strings.ToUpper(firstField(fields, "action"))

		switch {
		case !loggedIn && action != "LOGIN":
			// Pre-login guard: reject any non-LOGIN command with a KV failure (MOCK-02).
			// Does NOT consume from Expect — scripted responses are for post-login use.
			resp := s.buildFailureResponse("83000000010 Please login first")
			if err := WriteRRIFrame(conn, []byte(resp)); err != nil {
				return
			}

		case action == "LOGIN":
			if s.handleLogin(fields) {
				loggedIn = true
				resp := s.buildSuccessResponse()
				if err := WriteRRIFrame(conn, []byte(resp)); err != nil {
					return
				}
			} else {
				resp := s.buildFailureResponse("83000000010 Authentication failed")
				if err := WriteRRIFrame(conn, []byte(resp)); err != nil {
					return
				}
			}

		default:
			// Post-login scripted response path.
			s.mu.Lock()
			d := s.delay
			s.mu.Unlock()
			if d > 0 {
				time.Sleep(d)
			}

			select {
			case <-ctx.Done():
				return
			case item := <-s.Expect:
				if err := s.dispatch(conn, item); err != nil {
					return
				}
			}
			// Real DENIC servers close the connection after LOGOUT.
			// go-rriclient waits for the EOF after receiving the success response,
			// so we must close here or the client will block for 30 seconds.
			if action == "LOGOUT" {
				return
			}
		}
	}
}

// handleLogin validates the LOGIN command fields against stored MD5 hashes.
// Returns true if authentication succeeded.
func (s *MockRRIServer) handleLogin(fields map[string][]string) bool {
	user := firstField(fields, "user")
	// The incoming password field is already MD5-hashed by the client (DENIC protocol
	// requirement). The mock stores MD5 hashes and compares directly.
	pass := firstField(fields, "password")

	s.usersMu.RLock()
	defer s.usersMu.RUnlock()
	stored, ok := s.users[user]
	return ok && stored == pass
}

// buildSuccessResponse formats a KV success response with a unique STID.
func (s *MockRRIServer) buildSuccessResponse() string {
	return fmt.Sprintf("RESULT: success\nSTID: mock-%d\n", s.stidCounter.Add(1))
}

// buildFailureResponse formats a KV failure response with the given error message.
func (s *MockRRIServer) buildFailureResponse(errMsg string) string {
	return fmt.Sprintf("RESULT: failed\nSTID: mock-%d\nERROR: %s\n",
		s.stidCounter.Add(1), errMsg)
}

// dispatch sends one scripted response or applies a fault to conn.
func (s *MockRRIServer) dispatch(conn net.Conn, item any) error {
	switch v := item.(type) {
	case []byte:
		return WriteRRIFrame(conn, v) //nolint:wrapcheck // mock dispatch; conn.Write error is self-identifying at the test level

	case MalformedFrameFault:
		// A zero-length header: ReadRRIFrame explicitly rejects payloadLen == 0
		// ("empty message"), so the client must detect and reject this rather
		// than hang or misread a later frame as this one's body.
		var hdr [rriHeaderLen]byte
		_, err := conn.Write(hdr[:])
		return err //nolint:wrapcheck // mock fault inject; conn.Write error is self-identifying at the test level

	default:
		return fmt.Errorf("rri mock: unknown Expect item type %T", item)
	}
}
