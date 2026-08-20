//go:build unit

// Package conformance_test is the test harness for the prebuilt EPP/RRI
// conformance scenario library. Each test drives one scenario file against
// a mock EPP server and asserts the expected outcomes.
package conformance_test

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/config"
	epp_mock "github.com/riftcell/epp-test/pkg/mock/epp"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	"github.com/riftcell/epp-test/pkg/runner"
)

// ---- XML fixtures ----

// greetingXML is a minimal valid EPP greeting advertising domain, contact, and host services.
var greetingXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <greeting>
    <svID>Conformance Mock Server</svID>
    <svDate>2026-06-28T12:00:00.0Z</svDate>
    <svcMenu>
      <version>1.0</version>
      <lang>en</lang>
      <objURI>urn:ietf:params:xml:ns:domain-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:contact-1.0</objURI>
      <objURI>urn:ietf:params:xml:ns:host-1.0</objURI>
    </svcMenu>
  </greeting>
</epp>`)

// loginOKXML is the 1000 success response to an epp:login command.
var loginOKXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <trID><clTRID>conf-1</clTRID><svTRID>mock-001</svTRID></trID>
  </response>
</epp>`)

// successXML is a generic 1000 response for commands that do not return resData.
var successXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <trID><clTRID>conf-2</clTRID><svTRID>mock-002</svTRID></trID>
  </response>
</epp>`)

// domainCheckAvailXML is a domain:check response reporting conformance.example as available.
var domainCheckAvailXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:cd>
          <domain:name avail="1">conformance.example</domain:name>
        </domain:cd>
      </domain:chkData>
    </resData>
    <trID><clTRID>conf-3</clTRID><svTRID>mock-003</svTRID></trID>
  </response>
</epp>`)

// domainInfoXML is a domain:info response for conformance.example.
var domainInfoXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:infData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:name>conformance.example</domain:name>
        <domain:roid>DOMAIN-001</domain:roid>
        <domain:status s="ok"/>
        <domain:registrant>test-contact</domain:registrant>
        <domain:clID>conf-client</domain:clID>
      </domain:infData>
    </resData>
    <trID><clTRID>conf-4</clTRID><svTRID>mock-004</svTRID></trID>
  </response>
</epp>`)

// contactInfoXML is a contact:info response with email test@example.com.
var contactInfoXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <contact:infData xmlns:contact="urn:ietf:params:xml:ns:contact-1.0">
        <contact:id>test-contact</contact:id>
        <contact:postalInfo type="int">
          <contact:name>Test User</contact:name>
        </contact:postalInfo>
        <contact:email>test@example.com</contact:email>
        <contact:status s="ok"/>
      </contact:infData>
    </resData>
    <trID><clTRID>conf-5</clTRID><svTRID>mock-005</svTRID></trID>
  </response>
</epp>`)

// hostInfoXML is a host:info response for ns1.conformance.example.
var hostInfoXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <host:infData xmlns:host="urn:ietf:params:xml:ns:host-1.0">
        <host:name>ns1.conformance.example</host:name>
        <host:status s="ok"/>
        <host:addr ip="v4">192.0.2.1</host:addr>
      </host:infData>
    </resData>
    <trID><clTRID>conf-6</clTRID><svTRID>mock-006</svTRID></trID>
  </response>
</epp>`)

// pollMessageXML is a 1301 poll response with a pending message (ID: msg-conf-001).
// The msgQ.id attribute is used as PollMessage.ID for the poll_ack step.
var pollMessageXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1301"><msg>Command completed successfully; ack to dequeue</msg></result>
    <msgQ count="1" id="msg-conf-001">
      <qDate>2026-06-28T12:00:00.0Z</qDate>
      <msg>Transfer requested for conformance.example</msg>
    </msgQ>
    <trID><clTRID>conf-7</clTRID><svTRID>mock-007</svTRID></trID>
  </response>
</epp>`)

// pollEmptyXML is the 1300 "no messages" response signaling an empty queue.
var pollEmptyXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1300"><msg>Command completed successfully; no messages</msg></result>
    <trID><clTRID>conf-8</clTRID><svTRID>mock-008</svTRID></trID>
  </response>
</epp>`)

// ---- Test helpers ----

// newAdapter creates a MockEPPServer and a GenericEPPAdapter named "internetx"
// (matching the matrix entries in the conformance scenario files).
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

	// The adapter name must match a matrix entry in the conformance YAML files.
	// "internetx" is the first entry in matrix: [internetx, nicat, eurid].
	adapter, err := epp.NewGenericEPPAdapter("internetx", cfg, epp.WithTLSConfig(tlsCfg))
	require.NoError(t, err, "NewGenericEPPAdapter")

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return adapter, srv
}

// scriptLogin injects the greeting (via InjectPoll) and login-OK response (via Expect),
// then calls adapter.Login. Every test must call this before scripting operation responses.
func scriptLogin(t *testing.T, srv *epp_mock.MockEPPServer, adapter *epp.GenericEPPAdapter) {
	t.Helper()
	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML
	require.NoError(t, adapter.Login(context.Background()), "Login must succeed")
}

// ---- Conformance tests ----

// TestDomainLifecycle scripts the eight domain_lifecycle.yaml step responses and
// runs the scenario against the mock. CONF-01 / RFC 5731 §3.2.
//
// Response count after login:
//  1. check_domain        → domainCheckAvailXML (available:true)
//  2. create_contact      → successXML (ContactResult.ID from params with run-ID prefix)
//  3. create_domain       → successXML
//  4. info_domain         → domainInfoXML (Name: conformance.example)
//  5. update_domain       → successXML
//  6. renew_domain        → successXML
//  7. transfer_domain     → successXML
//  8. delete_domain       → successXML (explicit step)
//  9. cleanup delete_domain → successXML (t.Cleanup for create_domain)
//
// 10. cleanup delete_contact → successXML (t.Cleanup for create_contact).
func TestDomainLifecycle(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- domainCheckAvailXML // 1. check_domain
	srv.Expect <- successXML          // 2. create_contact
	srv.Expect <- successXML          // 3. create_domain
	srv.Expect <- domainInfoXML       // 4. info_domain (needs resData for Name assertion)
	srv.Expect <- successXML          // 5. update_domain
	srv.Expect <- successXML          // 6. renew_domain
	srv.Expect <- successXML          // 7. transfer_domain
	srv.Expect <- successXML          // 8. delete_domain (explicit step)
	srv.Expect <- successXML          // 9. cleanup: delete_domain
	srv.Expect <- successXML          // 10. cleanup: delete_contact

	runner.RunScenario(t, "domain_lifecycle.yaml", adapter)
}

// TestContactLifecycle scripts the six contact_lifecycle.yaml step responses and
// runs the scenario against the mock. CONF-02 / RFC 5733 §3.2.
//
// Response count after login:
//  1. create_contact     → successXML (ID captured from params)
//  2. info_contact       → contactInfoXML (Email: test@example.com)
//  3. update_contact     → successXML
//  4. create_domain      → successXML
//  5. delete_domain      → successXML (explicit step)
//  6. delete_contact     → successXML (explicit step)
//  7. cleanup delete_domain → successXML (t.Cleanup for create_domain)
//  8. cleanup delete_contact → successXML (t.Cleanup for create_contact)
func TestContactLifecycle(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML     // 1. create_contact
	srv.Expect <- contactInfoXML // 2. info_contact (Email assertion)
	srv.Expect <- successXML     // 3. update_contact
	srv.Expect <- successXML     // 4. create_domain
	srv.Expect <- successXML     // 5. delete_domain (explicit)
	srv.Expect <- successXML     // 6. delete_contact (explicit)
	srv.Expect <- successXML     // 7. cleanup: delete_domain
	srv.Expect <- successXML     // 8. cleanup: delete_contact

	runner.RunScenario(t, "contact_lifecycle.yaml", adapter)
}

// TestHostLifecycle scripts the four host_lifecycle.yaml step responses and
// runs the scenario against the mock. CONF-03 / RFC 5732 §3.2.
//
// Response count after login:
//  1. create_host     → successXML
//  2. info_host       → hostInfoXML (Name: ns1.conformance.example)
//  3. update_host     → successXML
//  4. delete_host     → successXML (explicit step)
//  5. cleanup delete_host → successXML (t.Cleanup for create_host)
func TestHostLifecycle(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML  // 1. create_host
	srv.Expect <- hostInfoXML // 2. info_host (Name assertion)
	srv.Expect <- successXML  // 3. update_host
	srv.Expect <- successXML  // 4. delete_host (explicit)
	srv.Expect <- successXML  // 5. cleanup: delete_host

	runner.RunScenario(t, "host_lifecycle.yaml", adapter)
}

// TestPollLifecycle scripts the poll_lifecycle.yaml responses and runs the scenario.
// CONF-04 / RFC 5730 §2.9.2.3.
//
// Response sequence after login:
//  1. wait_transfer (wait_for_poll): PollRead → pollMessageXML (1301, any type matches)
//  2. wait_transfer (wait_for_poll): PollAck  → successXML (internal ack inside wait_for_poll)
//  3. read (poll):                  PollRead  → pollMessageXML (1301, ID: msg-conf-001)
//  4. ack (poll_ack):               PollAck   → successXML
//  5. verify_empty (poll):          PollRead  → pollEmptyXML (1300 → EPPError{1300} → expect code 1300)
func TestPollLifecycle(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- pollMessageXML // 1. wait_for_poll: PollRead (message arrives, type matches empty)
	srv.Expect <- successXML     // 2. wait_for_poll: PollAck (internal ack)
	srv.Expect <- pollMessageXML // 3. read (poll): PollRead (message with ID for capture)
	srv.Expect <- successXML     // 4. ack (poll_ack): PollAck
	srv.Expect <- pollEmptyXML   // 5. verify_empty (poll): PollRead → 1300 (expect code 1300)

	runner.RunScenario(t, "poll_lifecycle.yaml", adapter)
}

// TestNegativeScenarios scripts the four negative error codes from negative_tests.yaml
// and asserts the runner correctly matches each expected code. CONF-07 / RFC 5730 §3.
//
// The runner registers t.Cleanup for any create_* op where the expect.code passes
// (even error codes 2302 and 2306 — the expect is "satisfied"), so delete cleanup is
// needed for the two create_domain steps even though they return error responses.
// Response count after login:
//  1. duplicate_create (create_domain → 2302)   — WrongResultCodeFault{2302}
//  2. not_found_info   (info_domain   → 2303)   — WrongResultCodeFault{2303}
//  3. auth_error       (update_domain → 2201)   — WrongResultCodeFault{2201}
//  4. bad_param        (create_domain → 2306)   — WrongResultCodeFault{2306}
//  5. cleanup: delete for bad_param domain      — successXML (reverse order: last create first)
//  6. cleanup: delete for duplicate_create domain — successXML
func TestNegativeScenarios(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- epp_mock.WrongResultCodeFault{Code: 2302} // 1. duplicate_create → object exists
	srv.Expect <- epp_mock.WrongResultCodeFault{Code: 2303} // 2. not_found_info → object does not exist
	srv.Expect <- epp_mock.WrongResultCodeFault{Code: 2201} // 3. auth_error → authorization error
	srv.Expect <- epp_mock.WrongResultCodeFault{Code: 2306} // 4. bad_param → parameter value policy error
	srv.Expect <- successXML                                // 5. cleanup: delete_domain (bad_param)
	srv.Expect <- successXML                                // 6. cleanup: delete_domain (duplicate_create)

	runner.RunScenario(t, "negative_tests.yaml", adapter)
}

// TestFaultTolerance verifies CONF-06: the adapter must not panic when the server
// sends a malformed frame. The check is exercised via the adapter directly because
// fault_tolerance.yaml has no expect block — a malformed frame produces a non-EPP
// Go error, and the runner would treat that as an unexpected error and fail the test.
// Calling the adapter directly lets us assert the exact non-nil error contract.
//
// MalformedFrameFault writes a 4-byte header with length=1 (below minimum valid
// EPP frame size of 4), causing ReadFrame to return an error without panicking.
//
// After the malformed frame, the adapter's withConn tries to reconnect (non-EPP error
// triggers the single-retry policy). We use a short-deadline context so the reconnect's
// DialContext fails fast after the 500ms backoff sleep, returning a wrapped error.
func TestFaultTolerance(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- epp_mock.MalformedFrameFault{}

	// Use a context with a short deadline so the reconnect attempt's DialContext
	// returns "context deadline exceeded" after the 500ms backoff sleep, without
	// deadlocking. The exact error type is irrelevant — we only assert non-nil.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Must return an error and must NOT panic.
	_, err := adapter.CheckDomain(ctx, "fault.example")
	assert.Error(t, err, "MalformedFrameFault must cause CheckDomain to return a non-nil error")
}
