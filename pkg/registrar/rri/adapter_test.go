//go:build unit

package rri_test

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net"
	"strconv"
	"testing"

	gorri "github.com/DENICeG/go-rriclient/pkg/rri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/config"
	rri_mock "github.com/riftcell/epp-test/pkg/mock/rri"
	"github.com/riftcell/epp-test/pkg/registrar"
	pkgrri "github.com/riftcell/epp-test/pkg/registrar/rri"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// md5HexTest is a local copy of md5Hex for use in test assertions.
// Mirrors the adapter's internal pre-hash so tests can compute the expected wire value.
func md5HexTest(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// plainTCPDialer returns a go-rriclient TLSDialer that uses plain TCP (no TLS).
// This allows connecting to MockRRIServer, which is plain TCP (not TLS).
func plainTCPDialer() gorri.TLSDialer {
	return func(network, addr string, _ *tls.Config) (gorri.TLSConnection, error) {
		return net.Dial(network, addr)
	}
}

// newTestAdapter creates a MockRRIServer and an RRIAdapter connected to it.
//
// The adapter uses a plain-TCP dialer so it can connect to the mock server
// (which does not use TLS). t.Cleanup handles server shutdown automatically.
func newTestAdapter(t *testing.T) (*rri_mock.MockRRIServer, *pkgrri.RRIAdapter) {
	t.Helper()

	srv := rri_mock.NewMockRRIServer(t)
	srv.AddUser("DENIC-1000", "secret")

	// Parse host and port from srv.Addr() for RegistrarConfig.
	host, portStr, err := net.SplitHostPort(srv.Addr())
	require.NoError(t, err, "parse server address")

	port, err := strconv.Atoi(portStr)
	require.NoError(t, err, "parse port number")

	cfg := config.RegistrarConfig{
		Host:     host,
		Port:     port,
		Username: "DENIC-1000",
		Password: "secret",
	}

	clientCfg := &gorri.ClientConfig{
		TLSDialHandler: plainTCPDialer(),
	}

	adapter, err := pkgrri.NewRRIAdapter(cfg, pkgrri.WithClientConfig(clientCfg))
	require.NoError(t, err, "create adapter")

	// Close the adapter's TCP connection at test teardown.
	// Without this, MockRRIServer's handleConn would wait up to 30 seconds
	// for the next frame before timing out, causing tests to appear to hang.
	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return srv, adapter
}

// drainReceived drains all currently buffered frames from the mock's Received channel.
// Used to clear the LOGIN frame before checking for a specific subsequent frame.
func drainReceived(srv *rri_mock.MockRRIServer) {
	for {
		select {
		case <-srv.Received:
		default:
			return
		}
	}
}

// TestLoginMD5OnWire proves that the LOGIN frame transmitted on the wire
// carries md5Hex("secret"), not the plaintext "secret".
//
// This is the RRI-04 wire-level proof required by the plan.
func TestLoginMD5OnWire(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed")

	// Drain the captured LOGIN frame from the mock server.
	var frame []byte
	select {
	case frame = <-srv.Received:
	default:
		t.Fatal("no frame received by mock server")
	}

	// Parse the KV frame to inspect the password field.
	fields := rri_mock.ParseKV(string(frame))

	// Assert the wire password is the MD5 hash, NOT the plaintext.
	expectedHash := md5HexTest("secret")
	wirePassword := fields["password"]
	require.Len(t, wirePassword, 1, "password field must appear exactly once")

	assert.Equal(t, expectedHash, wirePassword[0],
		"wire password must be md5Hex('secret'), not plaintext")
	assert.NotEqual(t, "secret", wirePassword[0],
		"wire password must NOT be plaintext")
}

// TestCheckDomainAvailable verifies that CheckDomain returns Available=true
// when the mock server responds with RESULT: success.
func TestCheckDomainAvailable(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	// Drain the LOGIN frame so it doesn't interfere with subsequent frame checks.
	drainReceived(srv)

	// Script the server to respond with a success (domain is available).
	srv.Expect <- []byte("RESULT: success\nSTID: mock-1\n")

	results, err := adapter.CheckDomain(context.Background(), "example.de")
	require.NoError(t, err, "CheckDomain must not error on success response")
	require.Len(t, results, 1, "must return one result")

	assert.Equal(t, "example.de", results[0].Name)
	require.NotNil(t, results[0].Available, "Available must be non-nil")
	assert.True(t, *results[0].Available, "Available must be true for success response")
}

// TestCreateDomainSuccess verifies that CreateDomain returns no error
// when the mock server responds with success.
func TestCreateDomainSuccess(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	drainReceived(srv)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-2\n")

	req := registrar.DomainCreateRequest{
		Name:        "newdomain.de",
		Registrant:  "DENIC-1000-JDOE",
		Nameservers: []string{"ns1.example.de", "ns2.example.de"},
	}

	result, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err, "CreateDomain must not error on success")
	assert.Equal(t, "newdomain.de", result.Name)
}

// TestRRIFailureMapsToEPPError verifies that a RESULT: failed response from
// the RRI server is surfaced as a *registrar.EPPError.
func TestRRIFailureMapsToEPPError(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	drainReceived(srv)

	// Script a failure response with a DENIC business code.
	srv.Expect <- []byte("RESULT: failed\nSTID: mock-3\nERROR: 83000000010 Authentication failed\n")

	_, err = adapter.CheckDomain(context.Background(), "taken.de")
	require.Error(t, err, "CheckDomain must return an error on RRI failure")

	var eppErr *registrar.EPPError
	assert.True(t, errors.As(err, &eppErr),
		"error must be (or wrap) *registrar.EPPError")
}

// TestTransferDomainRequestIssueTRANSIT verifies that TransferDomain with Op="request"
// sends a TRANSIT action query to the server.
func TestTransferDomainRequestIssueTRANSIT(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	// Drain the LOGIN frame first so the next Received frame is the TRANSIT query.
	drainReceived(srv)

	// Script success for the TRANSIT command.
	srv.Expect <- []byte("RESULT: success\nSTID: mock-4\n")

	req := registrar.DomainTransferRequest{
		Name: "transfer.de",
		Op:   "request",
	}

	_, err = adapter.TransferDomain(context.Background(), req)
	require.NoError(t, err, "TransferDomain request must succeed")

	// Check the captured frame to confirm it was a TRANSIT action.
	var frame []byte
	select {
	case frame = <-srv.Received:
	default:
		t.Fatal("no frame received by mock server after TransferDomain")
	}

	fields := rri_mock.ParseKV(string(frame))
	action := fields["action"]
	require.Len(t, action, 1, "action field must appear in the query")
	assert.Equal(t, "TRANSIT", action[0],
		"TransferDomain(op=request) must issue TRANSIT action")
}

// TestCreateHostReturnsCode2101 verifies that CreateHost returns a *EPPError
// with Code 2101 (command not implemented) without panicking.
func TestCreateHostReturnsCode2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	req := registrar.HostCreateRequest{Name: "ns1.example.de"}
	_, err = adapter.CreateHost(context.Background(), req)
	require.Error(t, err, "CreateHost must return an error")

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr),
		"error must be (or wrap) *registrar.EPPError")
	assert.Equal(t, 2101, eppErr.Code,
		"EPPError.Code must be 2101 (command not implemented)")
}
