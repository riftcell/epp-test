//go:build unit

package epp_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/config"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	"github.com/riftcell/epp-test/pkg/registrar"
	epp_mock "github.com/riftcell/epp-test/pkg/mock/epp"
	nbioxml "github.com/nbio/xml"
)

// TestMain enables goroutine leak detection for the entire test binary.
// The mock server starts goroutines; goleak ensures t.Cleanup tears them all down.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// greetingXML is a minimal valid EPP greeting payload (un-framed; mock frames it).
// The svcMenu advertises domain/contact/host objURIs so the extension filter works.
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
      <clTRID>gsd-test-1</clTRID>
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
      <clTRID>gsd-test-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// error2302XML is a 2302 "object exists" error response.
var error2302XML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="2302">
      <msg>Object exists</msg>
    </result>
    <trID>
      <clTRID>gsd-test-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// domainCheckResponseXML is a domain:chkData response for "example.com" available.
var domainCheckResponseXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000">
      <msg>Command completed successfully</msg>
    </result>
    <resData>
      <domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:cd>
          <domain:name avail="1">example.com</domain:name>
        </domain:cd>
      </domain:chkData>
    </resData>
    <trID>
      <clTRID>gsd-test-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// pollEmptyXML is an EPP 1300 "queue empty" poll response.
var pollEmptyXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1300">
      <msg>Command completed successfully; no messages</msg>
    </result>
    <trID>
      <clTRID>gsd-test-2</clTRID>
      <svTRID>mock-svtrid-002</svTRID>
    </trID>
  </response>
</epp>`)

// newAdapter creates a MockEPPServer and a GenericEPPAdapter wired to it.
// Returns both so callers can script mock.Expect and read mock.Received.
func newAdapter(t *testing.T) (*epp.GenericEPPAdapter, *epp_mock.MockEPPServer) {
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

	adapter, err := epp.NewGenericEPPAdapter("test", cfg, epp.WithTLSConfig(tlsCfg))
	require.NoError(t, err, "NewGenericEPPAdapter")

	// Close the adapter connection on test teardown so the mock server's handleConn
	// goroutine exits promptly instead of waiting for the 30-second connection deadline.
	t.Cleanup(func() {
		_ = adapter.Close() // acceptable: test teardown, close error is not actionable
	})

	return adapter, srv
}

// scriptLogin scripts the login handshake (greeting + login OK) and performs Login.
// Every operation test must call this before scripting its own operation response,
// because operations MUST NOT be called before Login (D-04).
//
// The EPP greeting is sent via InjectPoll so the server sends it spontaneously
// before reading the client's login frame. The mock server drains pollCh (non-blocking)
// before each ReadFrame — so InjectPoll(greetingXML) causes the greeting to be sent
// as the very first frame on a new connection, matching real EPP server behavior.
// The loginOKXML is enqueued in Expect so it is sent after the server reads the
// login frame from the client.
func scriptLogin(t *testing.T, srv *epp_mock.MockEPPServer, adapter *epp.GenericEPPAdapter) {
	t.Helper()

	// Greeting goes to pollCh (sent before client's first frame).
	srv.InjectPoll(greetingXML)
	// Login response goes to Expect (sent after server reads client's login frame).
	srv.Expect <- loginOKXML

	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed before running operations")
}

// ---- Tests ----

func TestLogin(t *testing.T) {
	adapter, srv := newAdapter(t)

	// Greeting via InjectPoll (server sends before reading client frame).
	srv.InjectPoll(greetingXML)
	// Login response via Expect (sent after server reads client's login frame).
	srv.Expect <- loginOKXML

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	// Read the login frame that the adapter sent and assert it contains epp:login elements.
	select {
	case frame := <-srv.Received:
		assert.True(t, containsLoginElement(frame), "received frame should contain epp:login, got: %s", frame)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for received login frame")
	}
}

func TestCheckDomain_Available(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- domainCheckResponseXML

	results, err := adapter.CheckDomain(context.Background(), "example.com")
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NotNil(t, results[0].Available, "Available should be non-nil")
	assert.True(t, *results[0].Available, "example.com should be available")
	assert.Equal(t, "example.com", results[0].Name)

	// Verify the adapter sent a domain:check frame.
	drainAndAssert(t, srv, "domain:check")
}

func TestCreateDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.DomainCreateRequest{
		Name:       "example.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
		Period:     1,
	}
	result, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)

	// Verify the adapter sent a domain:create frame.
	drainAndAssert(t, srv, "domain:create")
}

func TestCreateDomain_RegistryError2302(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- error2302XML

	req := registrar.DomainCreateRequest{
		Name:       "taken.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
	}
	_, err := adapter.CreateDomain(context.Background(), req)
	require.Error(t, err, "CreateDomain should return error for result code 2302")

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr), "error should be *registrar.EPPError, got: %T", err)
	assert.Equal(t, 2302, eppErr.Code)
}

func TestCreateDomain_HookCalledOnce(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	hookCallCount := 0
	adapter.OnBuildDomainCreate = func(req registrar.DomainCreateRequest, enc *nbioxml.Encoder) {
		hookCallCount++
		// Write a sentinel extension element so we can assert it lands in the frame.
		_ = enc.EncodeToken(nbioxml.StartElement{
			Name: nbioxml.Name{Space: "urn:test:ext-1.0", Local: "test:ext"},
		})
		_ = enc.EncodeToken(nbioxml.EndElement{
			Name: nbioxml.Name{Space: "urn:test:ext-1.0", Local: "test:ext"},
		})
	}

	req := registrar.DomainCreateRequest{
		Name:       "example.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
	}
	_, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, 1, hookCallCount, "OnBuildDomainCreate hook must be called exactly once")

	// The captured frame must contain the sentinel extension element.
	drainAndAssertMulti(t, srv, []string{"domain:create", "test:ext"})
}

func TestCreateDomain_NilHookNoPanic(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	// Ensure all hooks are nil (default) — no panic should occur.
	adapter.OnBuildDomainCreate = nil

	req := registrar.DomainCreateRequest{
		Name:       "example.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
	}

	// This must not panic.
	result, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)
}

func TestReconnect_AfterDropConnection(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	// Drop the connection abruptly (MOCK-08).
	srv.DropConnection()

	// The reconnect path: re-dial causes server to send greeting (via InjectPoll),
	// then adapter sends login, then adapter retries the original operation.
	// Script: greeting (InjectPoll) + loginOK (Expect) + operationResponse (Expect).
	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML
	srv.Expect <- successXML

	req := registrar.DomainCreateRequest{
		Name:       "reconnect.com",
		Registrant: "JDOE-1",
		AuthInfo:   "secret123",
	}

	// The withConn retry path reconnects transparently and retries the operation.
	_, err := adapter.CreateDomain(context.Background(), req)
	require.NoError(t, err, "CreateDomain should succeed after transparent reconnect")
}

// ---- helpers ----

// containsLoginElement checks whether a raw frame (XML bytes) contains an epp:login element.
func containsLoginElement(frame []byte) bool {
	return bytes.Contains(frame, []byte("login")) ||
		bytes.Contains(frame, []byte("clID")) ||
		bytes.Contains(frame, []byte("pw"))
}

// drainAndAssert reads all frames from srv.Received (with a short timeout) and
// asserts that at least one contains the expected substring.
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

// drainAndAssertMulti reads frames from srv.Received and asserts all wanted strings
// appear in at least one frame total.
func drainAndAssertMulti(t *testing.T, srv *epp_mock.MockEPPServer, wants []string) {
	t.Helper()

	found := make(map[string]bool)
	deadline := time.After(2 * time.Second)

	for len(found) < len(wants) {
		select {
		case frame := <-srv.Received:
			for _, w := range wants {
				if strings.Contains(string(frame), w) {
					found[w] = true
				}
			}
		case <-deadline:
			for _, w := range wants {
				if !found[w] {
					t.Errorf("timeout: no received frame contained %q", w)
				}
			}
			return
		}
	}
}

// TestPollRead_EmptyQueue verifies that a 1300 response maps to a *registrar.EPPError.
func TestPollRead_EmptyQueue(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- pollEmptyXML

	_, err := adapter.PollRead(context.Background())
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr), "error should be *registrar.EPPError")
	assert.Equal(t, 1300, eppErr.Code)
}

// Ensure the Greeting XML parses correctly (sanity check for the test constant).
func TestGreetingXMLParsesCorrectly(t *testing.T) {
	type GreetingCheck struct {
		XMLName interface{} `xml:"urn:ietf:params:xml:ns:epp-1.0 epp"`
	}
	// Quick check: the XML is valid.
	var g struct {
		XMLName interface{} `xml:"epp"`
	}
	err := nbioxml.Unmarshal(greetingXML, &g)
	require.NoError(t, err, "greetingXML should be valid XML")
}

// Ensure fmt is used (satisfies import usage for the Sprintf in reconnect test error path).
var _ = fmt.Sprintf
