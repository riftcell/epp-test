//go:build unit

package epp_test

import (
	"bytes"
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/mock/epp"
)

// clientTLSConfig returns a tls.Config for connecting to the mock server.
// Uses srv.ClientPool as RootCAs so InsecureSkipVerify stays false (MOCK-01).
// Includes a client cert generated from the server's CA for mTLS.
func clientTLSConfig(t *testing.T, srv *epp.MockEPPServer) *tls.Config {
	t.Helper()
	clientCert, err := epp.GenerateClientCert(srv.CAKey, srv.CACert)
	require.NoError(t, err)
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      srv.ClientPool,
		ServerName:   "127.0.0.1",
	}
}

// TestMockEPPScriptedResponse verifies the basic channel-push/receive API:
// enqueue a scripted response, dial TLS, send a frame, read the response back.
func TestMockEPPScriptedResponse(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	greetingXML := []byte(`<?xml version="1.0"?><epp><greeting/></epp>`)
	srv.Expect <- greetingXML

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Server reads incoming frame first, then dequeues and sends scripted response.
	require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))
	frame, err := epp.ReadFrame(conn)
	require.NoError(t, err)
	assert.Equal(t, greetingXML, frame)
}

// TestMockEPPReceivedCapture verifies that srv.Received captures client frames.
func TestMockEPPReceivedCapture(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	loginResponse := []byte(`<?xml version="1.0"?><epp><response><result code="1000"/></response></epp>`)
	srv.Expect <- loginResponse

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	loginXML := []byte(`<epp><command><login><clID>test</clID></login></command></epp>`)
	require.NoError(t, epp.WriteFrame(conn, loginXML))
	_, err = epp.ReadFrame(conn)
	require.NoError(t, err)

	select {
	case captured := <-srv.Received:
		assert.Equal(t, loginXML, captured, "srv.Received must capture the client's login frame")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for srv.Received to contain captured frame")
	}
}

// TestMockEPPMalformedFrameFault verifies that pushing MalformedFrameFault causes
// the client's ReadFrame to return an error (MOCK-05).
func TestMockEPPMalformedFrameFault(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	srv.Expect <- epp.MalformedFrameFault{}

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Send a trigger frame so the server reads and dispatches the fault.
	require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))

	_, err = epp.ReadFrame(conn)
	assert.Error(t, err, "MalformedFrameFault must cause ReadFrame to return an error")
}

// TestMockEPPWrongResultCodeFault verifies that WrongResultCodeFault returns a valid
// XML frame containing the requested result code (MOCK-06).
func TestMockEPPWrongResultCodeFault(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	srv.Expect <- epp.WrongResultCodeFault{Code: 2999}

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))
	frame, err := epp.ReadFrame(conn)
	require.NoError(t, err, "WrongResultCodeFault must return a well-framed response")
	assert.Contains(t, string(frame), `code="2999"`,
		"response must contain the requested result code")
}

// TestMockEPPDropConnection verifies that DropConnection causes the client's next
// read to fail with EOF or a closed-connection error (MOCK-08).
func TestMockEPPDropConnection(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Connect successfully, then drop.
	srv.DropConnection()

	// Client reads must now fail.
	_, err = epp.ReadFrame(conn)
	assert.Error(t, err, "DropConnection must cause subsequent client reads to fail")
}

// TestMockEPPSetDelay verifies that SetDelay introduces measurable latency (MOCK-08).
func TestMockEPPSetDelay(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	delay := 100 * time.Millisecond
	srv.SetDelay(delay)
	srv.Expect <- []byte(`<epp><response><result code="1000"/></response></epp>`)

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	start := time.Now()
	require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))
	_, err = epp.ReadFrame(conn)
	require.NoError(t, err)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, delay,
		"response must arrive at least %v after request (SetDelay)", delay)
}

// TestMockEPPInjectPoll verifies that InjectPoll sends an out-of-band frame
// before the next scripted response is processed (MOCK-07).
func TestMockEPPInjectPoll(t *testing.T) {
	srv := epp.NewMockEPPServer(t)

	pollXML := []byte(`<epp><response><result code="1301"/><msgQ/></response></epp>`)
	normalResponse := []byte(`<epp><response><result code="1000"/></response></epp>`)

	// Inject poll, then enqueue a normal response.
	srv.InjectPoll(pollXML)
	srv.Expect <- normalResponse

	conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Send one frame to trigger the dispatch loop.
	require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))

	// First read should be the poll injection (out-of-band before scripted response).
	firstFrame, err := epp.ReadFrame(conn)
	require.NoError(t, err)
	assert.Equal(t, pollXML, firstFrame, "InjectPoll frame must arrive before scripted response")

	// Second read should be the normal scripted response.
	secondFrame, err := epp.ReadFrame(conn)
	require.NoError(t, err)
	assert.Equal(t, normalResponse, secondFrame)
}

// TestMockEPPParallelInstances verifies zero goroutine leaks when many parallel
// tests each create, use, and let t.Cleanup close their own server (MOCK-03).
// goleak.VerifyTestMain in testmain_test.go validates leak-free shutdown.
func TestMockEPPParallelInstances(t *testing.T) {
	const n = 10
	for i := 0; i < n; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			srv := epp.NewMockEPPServer(t)

			response := []byte(`<epp><response><result code="1000"/></response></epp>`)
			srv.Expect <- response

			conn, err := tls.Dial("tcp", srv.Addr(), clientTLSConfig(t, srv))
			require.NoError(t, err)
			t.Cleanup(func() { conn.Close() })

			require.NoError(t, epp.WriteFrame(conn, []byte(`<epp><hello/></epp>`)))
			frame, err := epp.ReadFrame(conn)
			require.NoError(t, err)
			assert.Equal(t, response, frame)
			// t.Cleanup fires automatically — no manual Close needed
		})
	}
}

// TestMockEPPOffByFourRejection verifies the RFC 5734 off-by-four check:
// a frame whose 4-byte header encodes a total length of 3 (below the minimum
// of 4) must be rejected by ReadFrame (Phase 2 success criterion 5).
func TestMockEPPOffByFourRejection(t *testing.T) {
	// header=3 encodes a total length of 3, which is less than the minimum
	// valid EPP frame length of 4 (the header itself). ReadFrame must reject this.
	badFrame := []byte{0x00, 0x00, 0x00, 0x03} // total=3 < minimum 4
	_, err := epp.ReadFrame(bytes.NewReader(badFrame))
	assert.Error(t, err,
		"RFC 5734 guard: frame with total length < 4 must be rejected")
}
