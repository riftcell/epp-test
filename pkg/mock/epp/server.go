package epp

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// MockEPPServer is an in-process EPP mock server for unit testing.
//
// Create one per test using NewMockEPPServer(t). The constructor calls
// t.Cleanup automatically — do not call Close manually.
//
// Scripted response API:
//
//	srv.Expect <- xmlBytes                    // enqueue normal scripted response
//	srv.Expect <- MalformedFrameFault{}       // enqueue fault (MOCK-05)
//	srv.Expect <- WrongResultCodeFault{2999}  // enqueue bad result code (MOCK-06)
//	frame := <-srv.Received                   // read a captured client frame
//
// Connection-level fault injection:
//
//	srv.DropConnection()        // abrupt close (MOCK-08)
//	srv.SetDelay(50*time.Ms)    // configure response latency (MOCK-08)
//	srv.InjectPoll(xmlBytes)    // out-of-band unsolicited frame (MOCK-07)
type MockEPPServer struct {
	// ClientPool is the x509 CertPool containing the test CA.
	// Set this as RootCAs in the connecting client's tls.Config.
	// InsecureSkipVerify must remain false — use this pool instead.
	ClientPool *x509.CertPool

	// CAKey is the test CA private key. Pass to GenerateClientCert for mTLS.
	CAKey *ecdsa.PrivateKey

	// CACert is the test CA certificate. Pass to GenerateClientCert for mTLS.
	CACert *x509.Certificate

	// Expect is the scripted response queue. Test enqueues []byte for normal
	// responses or MalformedFrameFault / WrongResultCodeFault for fault injection.
	// Buffer size 32: large enough for typical scenarios; blocks if the test
	// enqueues more than 32 responses without the server consuming them.
	Expect chan any

	// Received captures every frame the server read from the client.
	// Buffer size 32: non-blocking send in handleConn; frames are dropped
	// if the buffer is full. Tests that assert on received frames must read
	// promptly or ensure scenario fits within the buffer.
	Received chan []byte

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	mu     sync.Mutex
	delay  time.Duration
	connMu sync.Mutex
	conn   net.Conn // active connection for DropConnection

	pollCh chan []byte // InjectPoll sends here; handleConn drains before Expect
}

// NewMockEPPServer creates a new in-process EPP mock server for the given test.
//
// The server listens on 127.0.0.1:0 (OS-assigned port) using TLS with mutual
// authentication (MOCK-01, MOCK-09). t.Cleanup is called automatically — do not
// call Close manually. goleak.VerifyTestMain in TestMain catches leaked goroutines
// if cleanup is missed.
func NewMockEPPServer(t interface {
	Helper()
	Fatal(...any)
	Cleanup(func())
}) *MockEPPServer {
	t.Helper()

	cert, pool, caKey, caCert, err := generateTestCert()
	if err != nil {
		t.Fatal("NewMockEPPServer: generateTestCert:", err)
	}

	// mTLS: require and verify client cert (MOCK-01).
	// Tests that need unauthenticated paths must supply their own tls.Listen.
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatal("NewMockEPPServer: tls.Listen:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv := &MockEPPServer{
		ClientPool: pool,
		CAKey:      caKey,
		CACert:     caCert,
		Expect:     make(chan any, 32),
		Received:   make(chan []byte, 32),
		pollCh:     make(chan []byte, 8),
		listener:   ln,
		cancel:     cancel,
	}

	srv.wg.Add(1)
	go srv.acceptLoop(ctx)

	// Automatic lifecycle (MOCK-03): test authors never call Close manually.
	// cancel() unblocks ctx.Done() in handleConn selects.
	// ln.Close() unblocks listener.Accept().
	// wg.Wait() drains all handler goroutines before the test exits.
	t.Cleanup(func() {
		cancel()
		_ = ln.Close() //nolint:errcheck // best-effort close in Cleanup; upstream error already captured, see CONVENTIONS.md §3
		srv.wg.Wait()
	})

	return srv
}

// Addr returns the server's TCP address in the form "127.0.0.1:PORT".
func (s *MockEPPServer) Addr() string {
	return s.listener.Addr().String()
}

// DropConnection closes the active client connection abruptly.
// The next read or write by the client will return io.EOF or net.ErrClosed (MOCK-08).
func (s *MockEPPServer) DropConnection() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close() //nolint:errcheck // best-effort close; method intent is abrupt connection drop, see CONVENTIONS.md §3
	}
}

// SetDelay configures a sleep duration applied before each scripted response.
// Subsequent calls overwrite the previous value. SetDelay(0) disables the delay.
// Safe to call concurrently (MOCK-08).
func (s *MockEPPServer) SetDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = d
}

// InjectPoll enqueues xmlPayload to be sent as an unsolicited out-of-band EPP frame
// before the next scripted response is processed. Safe to call from a test goroutine
// while a session is active (MOCK-07).
func (s *MockEPPServer) InjectPoll(xmlPayload []byte) {
	s.pollCh <- xmlPayload
}

func (s *MockEPPServer) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Listener was closed during t.Cleanup — normal shutdown path.
			return
		}
		s.connMu.Lock()
		s.conn = conn
		s.connMu.Unlock()

		s.wg.Add(1)
		go s.handleConn(ctx, conn)
	}
}

func (s *MockEPPServer) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close() //nolint:errcheck // best-effort close in defer; connection teardown on exit, see CONVENTIONS.md §3

	// 30-second deadline prevents the handler from hanging if the client panics
	// mid-test and never sends or closes the connection.
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}

	for {
		// Drain any pending poll injections before reading the next client frame.
		// Non-blocking: if pollCh is empty, proceed immediately.
		select {
		case pollPayload := <-s.pollCh:
			if err := WriteFrame(conn, pollPayload); err != nil {
				return
			}
		default:
		}

		frame, err := ReadFrame(conn)
		if err != nil {
			return
		}

		// Non-blocking send to Received: if the buffer is full, drop the frame.
		// Tests that assert on received frames must read promptly.
		select {
		case s.Received <- frame:
		default:
		}

		// Apply configurable delay before sending the scripted response (MOCK-08).
		s.mu.Lock()
		d := s.delay
		s.mu.Unlock()
		if d > 0 {
			time.Sleep(d)
		}

		// Dequeue the next scripted response. ctx.Done() case prevents the goroutine
		// from blocking forever if the test ends with items still in the queue.
		select {
		case <-ctx.Done():
			return
		case item := <-s.Expect:
			if err := s.dispatch(conn, item); err != nil {
				return
			}
		}
	}
}

// dispatch sends one scripted response or applies a fault to conn.
func (s *MockEPPServer) dispatch(conn net.Conn, item any) error {
	switch v := item.(type) {
	case []byte:
		return WriteFrame(conn, v)

	case MalformedFrameFault:
		// Write a 4-byte header with value 1 — impossible valid EPP frame
		// (minimum valid EPP frame length is 4). The client's ReadFrame must
		// return an error, not silently accept the frame (MOCK-05).
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], 1) // deliberate off-by-three
		_, err := conn.Write(hdr[:])
		return err //nolint:wrapcheck // mock fault inject; conn.Write error is self-identifying at the test level

	case WrongResultCodeFault:
		// Return a syntactically valid minimal EPP XML envelope with the
		// requested result code. Tests that the client handles unexpected
		// result codes without failing at the XML parse level (MOCK-06).
		xml := buildResultCodeResponse(v.Code)
		return WriteFrame(conn, xml)

	default:
		return fmt.Errorf("mock/epp: unknown Expect item type %T", item)
	}
}

// buildResultCodeResponse returns a minimal valid EPP XML envelope containing
// only a <result code="N"> element. Used by WrongResultCodeFault dispatch.
func buildResultCodeResponse(code int) []byte {
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`+
			`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">`+
			`<response>`+
			`<result code="%d">`+
			`<msg>Mock result code %d</msg>`+
			`</result>`+
			`<trID><svTRID>mock-svtrid-001</svTRID></trID>`+
			`</response>`+
			`</epp>`,
		code, code,
	))
}
