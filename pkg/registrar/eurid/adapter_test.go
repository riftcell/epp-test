//go:build unit

package eurid_test

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
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	"github.com/riftcell/epp-test/pkg/registrar/eurid"
	"github.com/riftcell/epp-test/pkg/registrar"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// greetingXML is a minimal valid EPP greeting payload.
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
      <clTRID>gsd-eurid-1</clTRID>
      <svTRID>mock-svtrid-001</svTRID>
    </trID>
  </response>
</epp>`)

// successXML is a generic 1000 response.
var successXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <trID>
      <clTRID>gsd-eurid-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// newAdapter creates a MockEPPServer and an EURidAdapter wired to it.
func newAdapter(t *testing.T) (*eurid.EURidAdapter, *epp_mock.MockEPPServer) {
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

	adapter, err := eurid.NewEURidAdapter(cfg, epp.WithTLSConfig(tlsCfg))
	require.NoError(t, err, "NewEURidAdapter")

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return adapter, srv
}

// scriptLogin scripts the login handshake and performs Login.
func scriptLogin(t *testing.T, srv *epp_mock.MockEPPServer, adapter *eurid.EURidAdapter) {
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

// TestName verifies EURidAdapter.Name() returns "eurid".
func TestName(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)
	assert.Equal(t, "eurid", adapter.Name())
}

// TestLogin_Success verifies login succeeds against MockEPPServer.
func TestLogin_Success(t *testing.T) {
	adapter, srv := newAdapter(t)

	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed")
}

// TestCreateContact_NoPanic proves CreateContact with the contact-ext-1.3 hook set
// does not panic.
func TestCreateContact_NoPanic(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.ContactCreateRequest{
		ID:          "EURID-1",
		Name:        "Jane Smith",
		Email:       "jane@example.eu",
		CountryCode: "BE",
		AuthInfo:    "secret",
	}

	result, err := adapter.CreateContact(context.Background(), req)
	require.NoError(t, err, "CreateContact must not error on success response")
	assert.Equal(t, "EURID-1", result.ID)
}

// TestCreateDomain_ExtensionInFrame proves CreateDomain with the domain-ext + nsgroup
// hooks set produces a frame that contains an epp:extension element.
// This is the REG-04 must-have for stub hooks.
func TestCreateDomain_ExtensionInFrame(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.DomainCreateRequest{
		Name:       "example.eu",
		Registrant: "EURID-1",
		AuthInfo:   "secret",
		Period:     1,
	}

	// CreateDomain with the buildDomainExt hook must not panic.
	result, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err, "CreateDomain must not error on success response")
	assert.Equal(t, "example.eu", result.Name)

	// The captured domain:create frame must contain an extension element.
	drainAndAssert(t, srv, "extension")
}
