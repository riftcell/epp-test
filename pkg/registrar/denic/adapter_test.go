//go:build unit

package denic_test

import (
	"context"
	"crypto/tls"
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
	"github.com/riftcell/epp-test/pkg/registrar/denic"
	pkgrri "github.com/riftcell/epp-test/pkg/registrar/rri"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// plainTCPDialer returns a go-rriclient TLSDialer that uses plain TCP (no TLS).
// This allows connecting to MockRRIServer, which is plain TCP (not TLS).
func plainTCPDialer() gorri.TLSDialer {
	return func(network, addr string, _ *tls.Config) (gorri.TLSConnection, error) {
		return net.Dial(network, addr)
	}
}

// newAdapter creates a MockRRIServer and a DENICAdapter connected to it.
//
// The adapter uses a plain-TCP dialer so it can connect to the mock server
// (which does not use TLS). t.Cleanup handles server shutdown automatically.
func newAdapter(t *testing.T) (*rri_mock.MockRRIServer, *denic.DENICAdapter) {
	t.Helper()

	srv := rri_mock.NewMockRRIServer(t)
	srv.AddUser("DENIC-1000", "secret")

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

	adapter, err := denic.NewDENICAdapter(cfg, pkgrri.WithClientConfig(clientCfg))
	require.NoError(t, err, "create adapter")

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return srv, adapter
}

// TestName verifies DENICAdapter.Name() returns "denic" (inherited from RRIAdapter).
func TestName(t *testing.T) {
	_, adapter := newAdapter(t)
	assert.Equal(t, "denic", adapter.Name())
}

// TestLogin_Success verifies Login against MockRRIServer with MD5-hashed password succeeds.
// This proves the inherited RRI login behavior works through the DENICAdapter wrapper.
func TestLogin_Success(t *testing.T) {
	_, adapter := newAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed with correct credentials")
}

// TestCreateHost_ReturnsEPPError2101 verifies that CreateHost returns an inherited
// *registrar.EPPError with Code 2101 without panicking.
// DENIC RRI has no host objects — the RRIAdapter returns EPPError{Code:2101}
// for all host operations. DENICAdapter inherits this behavior (REG-05).
func TestCreateHost_ReturnsEPPError2101(t *testing.T) {
	_, adapter := newAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")

	req := registrar.HostCreateRequest{
		Name: "ns1.example.de",
	}

	_, createErr := adapter.CreateHost(context.Background(), req)
	require.Error(t, createErr, "CreateHost must return an error on DENIC")

	var eppErr *registrar.EPPError
	require.True(t, errors.As(createErr, &eppErr),
		"error must be *registrar.EPPError, got: %T", createErr)
	assert.Equal(t, 2101, eppErr.Code, "DENIC host operation must return EPPError{Code:2101}")
}
