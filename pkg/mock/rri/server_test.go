//go:build unit

package rri_test

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/mock/rri"
)

// md5HexTest computes the MD5 hex digest of s for use in test LOGIN frames.
// This mirrors what the DENIC Phase 3 adapter must do before calling NewLoginQuery.
func md5HexTest(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// sendRRIFrame writes a single RRI frame to conn (4-byte payload-length header + payload).
func sendRRIFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	require.NoError(t, rri.WriteRRIFrame(conn, payload))
}

// recvRRIFrame reads a single RRI frame from conn.
func recvRRIFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	frame, err := rri.ReadRRIFrame(conn)
	require.NoError(t, err)
	return frame
}

// loginFrame constructs a KV LOGIN frame with the password pre-hashed as MD5.
func loginFrame(username, plaintextPassword string) []byte {
	return []byte(fmt.Sprintf(
		"version: 5.0\naction: LOGIN\nuser: %s\npassword: %s\n",
		username, md5HexTest(plaintextPassword),
	))
}

// TestRRIMockLoginSuccess verifies that a correctly authenticated LOGIN
// returns a KV success response and advances to logged-in state (MOCK-02).
func TestRRIMockLoginSuccess(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "s3cr3t")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "s3cr3t"))
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: success",
		"correct MD5-hashed credentials must produce a success response")
}

// TestRRIMockLoginWrongPassword verifies that a wrong password returns a KV
// failure and does NOT advance to logged-in state.
func TestRRIMockLoginWrongPassword(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "correctpassword")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Send LOGIN with wrong password
	wrongLoginFrame := []byte(fmt.Sprintf(
		"version: 5.0\naction: LOGIN\nuser: DENIC-Test\npassword: %s\n",
		md5HexTest("wrongpassword"),
	))
	sendRRIFrame(t, conn, wrongLoginFrame)
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: failed",
		"wrong MD5-hashed password must produce a failure response")

	// After failed login, send a non-LOGIN command — must still be rejected
	srv.Expect <- []byte("RESULT: success\nSTID: 1\n") // enqueue a scripted response
	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	resp2 := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp2), "RESULT: failed",
		"command after failed login must be rejected (login state not advanced)")
}

// TestRRIMockPreLoginRejection verifies that non-LOGIN commands before authentication
// return KV failure responses without consuming from srv.Expect (MOCK-02).
func TestRRIMockPreLoginRejection(t *testing.T) {
	srv := rri.NewMockRRIServer(t)

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Send a CHECK command before login
	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: failed",
		"pre-login non-LOGIN command must be rejected")
	assert.Contains(t, string(resp), "Please login first",
		"rejection must include a clear reason")

	// Expect channel must still be empty (pre-login rejection does not consume from Expect)
	select {
	case <-srv.Expect:
		t.Fatal("pre-login rejection must NOT consume from Expect channel")
	default:
		// correct: Expect was not consumed
	}
}

// TestRRIMockScriptedResponsePostLogin verifies that after successful login,
// commands receive scripted responses from srv.Expect (MOCK-04).
func TestRRIMockScriptedResponsePostLogin(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "password")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Login
	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "password"))
	loginResp := recvRRIFrame(t, conn)
	require.Contains(t, string(loginResp), "RESULT: success")

	// Enqueue a scripted CHECK response
	checkResponse := []byte("RESULT: success\nSTID: check-001\nDOMAIN: available\n")
	srv.Expect <- checkResponse

	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	resp := recvRRIFrame(t, conn)
	assert.Equal(t, checkResponse, resp,
		"post-login command must receive the queued scripted response")
}

// TestRRIMockReceivedCapture verifies that srv.Received captures client frames.
func TestRRIMockReceivedCapture(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "password")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	loginPayload := loginFrame("DENIC-Test", "password")
	sendRRIFrame(t, conn, loginPayload)
	recvRRIFrame(t, conn) // consume the login response

	select {
	case captured := <-srv.Received:
		assert.Equal(t, loginPayload, captured,
			"srv.Received must capture the LOGIN frame")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for srv.Received to contain captured frame")
	}
}

// TestRRIMockSetDelay verifies that SetDelay introduces measurable latency (MOCK-08).
func TestRRIMockSetDelay(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "password")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Login first
	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "password"))
	recvRRIFrame(t, conn)

	delay := 100 * time.Millisecond
	srv.SetDelay(delay)
	srv.Expect <- []byte("RESULT: success\nSTID: 1\n")

	start := time.Now()
	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	recvRRIFrame(t, conn)
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, delay,
		"response must arrive at least %v after request (SetDelay)", delay)
}

// TestRRIMockParallelInstances verifies zero goroutine leaks when many parallel
// tests each create, use, and let t.Cleanup close their own server (MOCK-03).
func TestRRIMockParallelInstances(t *testing.T) {
	const n = 10
	for i := 0; i < n; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			srv := rri.NewMockRRIServer(t)
			srv.AddUser("user", "pass")

			conn, err := net.Dial("tcp", srv.Addr())
			require.NoError(t, err)
			t.Cleanup(func() { conn.Close() })

			sendRRIFrame(t, conn, loginFrame("user", "pass"))
			resp := recvRRIFrame(t, conn)
			assert.Contains(t, string(resp), "RESULT: success")
			// t.Cleanup fires automatically — no manual Close needed
		})
	}
}

// TestRRIMockMD5PasswordNotPlaintext verifies the critical MD5 property:
// AddUser stores a hash; a client sending plaintext (not MD5) must be rejected.
// This confirms Pitfall 5 is avoided in the mock implementation.
func TestRRIMockMD5PasswordNotPlaintext(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "secret")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	// Send plaintext password (not MD5-hashed) — must fail
	plaintextLoginFrame := []byte("version: 5.0\naction: LOGIN\nuser: DENIC-Test\npassword: secret\n")
	sendRRIFrame(t, conn, plaintextLoginFrame)
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: failed",
		"plaintext password must be rejected — client must MD5-hash before sending")
}

// TestRRIMockTLSLoginAndScriptedResponse verifies NewMockRRIServerTLS speaks
// the same protocol as NewMockRRIServer (login state, scripted responses)
// over an actual TLS connection, for clients whose dial hook requires a
// concrete *tls.Conn (e.g. wcs-externalapi's rri.Client) rather than the
// generic connection interface a plain-TCP dial hook can satisfy.
func TestRRIMockTLSLoginAndScriptedResponse(t *testing.T) {
	srv := rri.NewMockRRIServerTLS(t)
	srv.AddUser("DENIC-Test", "s3cr3t")

	// InsecureSkipVerify mirrors how DENIC RRI clients (including
	// wcs-externalapi's, via ClientConfig.Insecure) actually dial: the
	// self-signed certificate is never validated.
	conn, err := tls.Dial("tcp", srv.Addr(), &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "s3cr3t"))
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: success",
		"correct MD5-hashed credentials over TLS must produce a success response")

	srv.Expect <- []byte("RESULT: success\nSTID: tls-1\n")
	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	resp2 := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp2), "STID: tls-1",
		"a scripted post-login response must be delivered over TLS the same as over plain TCP")
}

// TestRRIMockTLSRejectsPreLoginCommand verifies the pre-login guard (MOCK-02)
// applies identically over TLS.
func TestRRIMockTLSRejectsPreLoginCommand(t *testing.T) {
	srv := rri.NewMockRRIServerTLS(t)

	conn, err := tls.Dial("tcp", srv.Addr(), &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))
	resp := recvRRIFrame(t, conn)
	assert.Contains(t, string(resp), "RESULT: failed",
		"a command before LOGIN must be rejected over TLS the same as over plain TCP")
}

// TestRRIMockDropConnection verifies that DropConnection causes the client's
// next read to fail with EOF or a closed-connection error.
func TestRRIMockDropConnection(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "s3cr3t")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "s3cr3t"))
	resp := recvRRIFrame(t, conn)
	require.Contains(t, string(resp), "RESULT: success")

	srv.DropConnection()

	_, err = rri.ReadRRIFrame(conn)
	assert.Error(t, err, "DropConnection must cause the client's subsequent read to fail")
}

// TestRRIMockMalformedFrameFault verifies that pushing MalformedFrameFault
// causes the client's ReadRRIFrame to return an error rather than hanging or
// misparsing a subsequent frame.
func TestRRIMockMalformedFrameFault(t *testing.T) {
	srv := rri.NewMockRRIServer(t)
	srv.AddUser("DENIC-Test", "s3cr3t")

	conn, err := net.Dial("tcp", srv.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	sendRRIFrame(t, conn, loginFrame("DENIC-Test", "s3cr3t"))
	resp := recvRRIFrame(t, conn)
	require.Contains(t, string(resp), "RESULT: success")

	srv.Expect <- rri.MalformedFrameFault{}
	sendRRIFrame(t, conn, []byte("version: 5.0\naction: CHECK\ndomain: example.de\n"))

	_, err = rri.ReadRRIFrame(conn)
	assert.Error(t, err, "MalformedFrameFault must cause ReadRRIFrame to return an error")
}
