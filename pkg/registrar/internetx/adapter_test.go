//go:build unit

package internetx_test

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/config"
	epp_mock "github.com/riftcell/epp-test/pkg/mock/epp"
	"github.com/riftcell/epp-test/pkg/registrar"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	"github.com/riftcell/epp-test/pkg/registrar/internetx"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// greetingXML is a minimal valid EPP greeting payload (un-framed; mock frames it).
// The svcMenu advertises domain/contact/host objURIs so the adapter's extension filter works.
var greetingXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <greeting>
    <svID>EPP Mock Server</svID>
    <svDate>2026-06-27T12:00:00.0Z</svDate>
    <svcMenu>
      <version>1.0</version>
      <lang>en</lang>
      <objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:host-1.0</objURI>
    </svcMenu>
  </greeting>
</epp>`)

// loginOKXML is a result code 1000 response to the epp:login command.
var loginOKXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <trID>
      <clTRID>gsd-internetx-1</clTRID>
      <svTRID>mock-svtrid-001</svTRID>
    </trID>
  </response>
</epp>`)

// successXML is a generic 1000 response for commands that don't return resData.
var successXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <trID>
      <clTRID>gsd-internetx-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// newAdapter creates a MockEPPServer and an InternetXAdapter wired to it.
func newAdapter(t *testing.T) (*internetx.InternetXAdapter, *epp_mock.MockEPPServer) {
	t.Helper()

	srv := epp_mock.NewMockEPPServer(t)

	clientCert, err := epp_mock.GenerateClientCert(srv.CAKey, srv.CACert)
	require.NoError(t, err, "GenerateClientCert")

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      srv.ClientPool,
		ServerName:   "127.0.0.1",
	}

	host, portStr, err := net.SplitHostPort(srv.Addr())
	require.NoError(t, err, "parse mock server addr")
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err, "parse port")

	cfg := config.RegistrarConfig{
		Host:     host,
		Port:     port,
		Username: "u",
		Password: "p",
	}

	adapter, err := internetx.NewInternetXAdapter(cfg, epp.WithTLSConfig(tlsCfg))
	require.NoError(t, err, "NewInternetXAdapter")

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return adapter, srv
}

// scriptLogin scripts the login handshake (greeting + login OK) and performs Login.
func scriptLogin(t *testing.T, srv *epp_mock.MockEPPServer, adapter *internetx.InternetXAdapter) {
	t.Helper()
	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed before running operations")
}

// drainAndAssert reads frames from srv.Received and asserts at least one contains want.
func drainAndAssert(t *testing.T, srv *epp_mock.MockEPPServer, want string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case frame := <-srv.Received:
			if strings.Contains(string(frame), want) {
				return
			}
		case <-deadline:
			t.Errorf("timeout: no received frame contained %q", want)
			return
		}
	}
}

// TestName verifies InternetXAdapter.Name() returns "internetx".
func TestName(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)
	assert.Equal(t, "internetx", adapter.Name())
}

// TestLogin_Success verifies login succeeds against MockEPPServer.
func TestLogin_Success(t *testing.T) {
	adapter, srv := newAdapter(t)

	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed")
}

// TestCreateDomain_NilHooksNoPanic proves CreateDomain with all hooks nil does not panic.
// This is the REG-02 must-have: operations without extension hooks must be safe.
func TestCreateDomain_NilHooksNoPanic(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.DomainCreateRequest{
		Name:       "example.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
		Period:     1,
	}

	// All hooks are nil by default in InternetXAdapter — must not panic.
	result, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err, "CreateDomain with nil hooks must not error on success response")
	assert.Equal(t, "example.com", result.Name)

	// The sent frame must contain domain:create.
	drainAndAssert(t, srv, "domain:create")
}
