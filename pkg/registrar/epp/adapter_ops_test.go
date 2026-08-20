//go:build unit

package epp_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/config"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	"github.com/riftcell/epp-test/pkg/registrar"
)

// ---- XML response templates for additional operations ----

var logoutOKXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1500">
      <msg>Command completed successfully; ending session</msg>
    </result>
    <trID>
      <clTRID>gsd-test-3</clTRID>
      <svTRID>mock-3</svTRID>
    </trID>
  </response>
</epp>`)

var domainInfoResponseXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:infData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:name>example.com</domain:name>
        <domain:roid>EXAMPLE1-COM</domain:roid>
        <domain:status s="ok"/>
        <domain:registrant>JDOE-1</domain:registrant>
        <domain:crDate>2024-01-15T10:00:00Z</domain:crDate>
        <domain:exDate>2025-01-15T10:00:00Z</domain:exDate>
      </domain:infData>
    </resData>
    <trID><clTRID>gsd-test-5</clTRID><svTRID>mock-5</svTRID></trID>
  </response>
</epp>`)

var contactInfoResponseXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <contact:infData xmlns:contact="urn:ietf:params:xml:ns:contact-1.0">
        <contact:id>JDOE-1</contact:id>
        <contact:status s="ok"/>
        <contact:postalInfo type="int">
          <contact:name>John Doe</contact:name>
          <contact:org>Example Inc</contact:org>
          <contact:addr>
            <contact:street>123 Main St</contact:street>
            <contact:city>Vienna</contact:city>
            <contact:cc>AT</contact:cc>
          </contact:addr>
        </contact:postalInfo>
        <contact:email>jdoe@example.com</contact:email>
        <contact:crDate>2024-01-15T10:00:00Z</contact:crDate>
      </contact:infData>
    </resData>
    <trID><clTRID>gsd-test-6</clTRID><svTRID>mock-6</svTRID></trID>
  </response>
</epp>`)

var hostInfoResponseXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <host:infData xmlns:host="urn:ietf:params:xml:ns:host-1.0">
        <host:name>ns1.example.com</host:name>
        <host:status s="ok"/>
        <host:addr ip="v4">93.184.216.34</host:addr>
      </host:infData>
    </resData>
    <trID><clTRID>gsd-test-7</clTRID><svTRID>mock-7</svTRID></trID>
  </response>
</epp>`)

var pollMessageXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1301"><msg>Command completed successfully; ack to dequeue</msg></result>
    <msgQ count="1" id="msg-001">
      <msg>Transfer requested for example.com</msg>
    </msgQ>
    <trID><clTRID>gsd-test-8</clTRID><svTRID>mock-8</svTRID></trID>
  </response>
</epp>`)

var domainCheckNotAvailableXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:cd>
          <domain:name avail="0">taken.com</domain:name>
        </domain:cd>
      </domain:chkData>
    </resData>
    <trID><clTRID>gsd-test-2</clTRID><svTRID>mock-svtrid-002</svTRID></trID>
  </response>
</epp>`)

var domainCheckMultipleXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0" xmlns:epp="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:cd>
          <domain:name avail="1">free.com</domain:name>
        </domain:cd>
        <domain:cd>
          <domain:name avail="0">taken.com</domain:name>
        </domain:cd>
      </domain:chkData>
    </resData>
    <trID><clTRID>gsd-test-2</clTRID><svTRID>mock-svtrid-002</svTRID></trID>
  </response>
</epp>`)

// ---- Name ----

func TestName(t *testing.T) {
	adapter, _ := newAdapter(t)
	assert.Equal(t, "test", adapter.Name())
}

// ---- Logout ----

func TestLogout_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- logoutOKXML

	err := adapter.Logout(context.Background())
	require.NoError(t, err)

	drainAndAssert(t, srv, "logout")
}

// ---- Ping ----

func TestPing_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	// Ping sends <hello>; the mock reads it and dequeues the next Expect item to send back.
	// The adapter reads that response as the greeting. We send back greetingXML.
	srv.Expect <- greetingXML

	err := adapter.Ping(context.Background())
	require.NoError(t, err)
}

// ---- InfoDomain ----

func TestInfoDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- domainInfoResponseXML

	result, err := adapter.InfoDomain(context.Background(), "example.com")
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)
	assert.Equal(t, "JDOE-1", result.Registrant)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.ExpiresAt)

	drainAndAssert(t, srv, "domain:info")
}

// ---- UpdateDomain ----

func TestUpdateDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.DomainUpdateRequest{
		Name:           "example.com",
		AddNameservers: []string{"ns3.example.com"},
	}
	result, err := adapter.UpdateDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)

	drainAndAssert(t, srv, "domain:update")
}

// ---- DeleteDomain ----

func TestDeleteDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	err := adapter.DeleteDomain(context.Background(), "example.com")
	require.NoError(t, err)

	drainAndAssert(t, srv, "domain:delete")
}

// ---- RenewDomain ----

func TestRenewDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	result, err := adapter.RenewDomain(context.Background(), "example.com", 2)
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)

	drainAndAssert(t, srv, "domain:renew")
}

// ---- TransferDomain ----

func TestTransferDomain_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.DomainTransferRequest{
		Name:     "example.com",
		Op:       "request",
		AuthInfo: "secret",
	}
	result, err := adapter.TransferDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.com", result.Name)

	drainAndAssert(t, srv, "domain:transfer")
}

// ---- CheckDomain edge cases ----

func TestCheckDomain_NotAvailable(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- domainCheckNotAvailableXML

	results, err := adapter.CheckDomain(context.Background(), "taken.com")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Available)
	assert.False(t, *results[0].Available, "taken.com should not be available")
}

func TestCheckDomain_MultipleNames(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- domainCheckMultipleXML

	results, err := adapter.CheckDomain(context.Background(), "free.com", "taken.com")
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.NotNil(t, results[0].Available)
	assert.True(t, *results[0].Available, "free.com should be available")
	require.NotNil(t, results[1].Available)
	assert.False(t, *results[1].Available, "taken.com should not be available")
}

// ---- CheckContact ----

func TestCheckContact_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	results, err := adapter.CheckContact(context.Background(), "JDOE-1")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "JDOE-1", results[0].ID)

	drainAndAssert(t, srv, "contact:check")
}

// ---- InfoContact ----

func TestInfoContact_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- contactInfoResponseXML

	result, err := adapter.InfoContact(context.Background(), "JDOE-1")
	require.NoError(t, err)
	assert.Equal(t, "JDOE-1", result.ID)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, "jdoe@example.com", result.Email)

	drainAndAssert(t, srv, "contact:info")
}

// ---- CreateContact ----

func TestCreateContact_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.ContactCreateRequest{
		ID:          "JDOE-1",
		Name:        "John Doe",
		Email:       "jdoe@example.com",
		Street:      []string{"123 Main St"},
		City:        "Vienna",
		CountryCode: "AT",
		AuthInfo:    "secret",
	}
	result, err := adapter.CreateContact(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "JDOE-1", result.ID)

	drainAndAssert(t, srv, "contact:create")
}

// ---- UpdateContact ----

func TestUpdateContact_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.ContactUpdateRequest{
		ID:    "JDOE-1",
		Email: "new@example.com",
	}
	result, err := adapter.UpdateContact(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "JDOE-1", result.ID)

	drainAndAssert(t, srv, "contact:update")
}

// ---- DeleteContact ----

func TestDeleteContact_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	err := adapter.DeleteContact(context.Background(), "JDOE-1")
	require.NoError(t, err)

	drainAndAssert(t, srv, "contact:delete")
}

// ---- CheckHost ----

func TestCheckHost_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	results, err := adapter.CheckHost(context.Background(), "ns1.example.com")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "ns1.example.com", results[0].Name)

	drainAndAssert(t, srv, "host:check")
}

// ---- InfoHost ----

func TestInfoHost_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- hostInfoResponseXML

	result, err := adapter.InfoHost(context.Background(), "ns1.example.com")
	require.NoError(t, err)
	assert.Equal(t, "ns1.example.com", result.Name)
	assert.Len(t, result.Addrs, 1)

	drainAndAssert(t, srv, "host:info")
}

// ---- CreateHost ----

func TestCreateHost_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.HostCreateRequest{
		Name:  "ns1.example.com",
		Addrs: []string{"93.184.216.34"},
	}
	result, err := adapter.CreateHost(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "ns1.example.com", result.Name)

	drainAndAssert(t, srv, "host:create")
}

// ---- UpdateHost ----

func TestUpdateHost_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.HostUpdateRequest{
		Name:     "ns1.example.com",
		AddAddrs: []string{"1.2.3.4"},
	}
	result, err := adapter.UpdateHost(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "ns1.example.com", result.Name)

	drainAndAssert(t, srv, "host:update")
}

// ---- DeleteHost ----

func TestDeleteHost_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	err := adapter.DeleteHost(context.Background(), "ns1.example.com")
	require.NoError(t, err)

	drainAndAssert(t, srv, "host:delete")
}

// ---- PollRead WithMessage ----

func TestPollRead_WithMessage(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- pollMessageXML

	msg, err := adapter.PollRead(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "msg-001", msg.ID)
	assert.Equal(t, 1, msg.QueueDepth)
	assert.NotEmpty(t, msg.Content)
}

// ---- PollAck ----

func TestPollAck_Success(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	err := adapter.PollAck(context.Background(), "msg-001")
	require.NoError(t, err)

	// The poll:ack XML frame contains op="ack" and the msgID.
	drainAndAssertMulti(t, srv, []string{"ack", "msg-001"})
}

// ---- Error type assertion for registry error ----

func TestDeleteDomain_RegistryError(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- error2302XML

	err := adapter.DeleteDomain(context.Background(), "example.com")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2302, eppErr.Code)
}

// ---- Validation-wiring tests (SEC-03) ----
// These tests confirm that invalid input returns EPPError code 2005 BEFORE
// any connection is needed (no mock server required).

// mustRegistrarConfig returns a minimal RegistrarConfig for no-connection tests.
// The adapter is never dialed — validation returns before withConn is called.
func mustRegistrarConfig() config.RegistrarConfig {
	return config.RegistrarConfig{
		Host:     "127.0.0.1",
		Port:     9999,
		Username: "u",
		Password: "p",
	}
}

func TestCheckDomain_InvalidName_ReturnsCode2005(t *testing.T) {
	// newAdapterNoLogin creates an adapter with NO mock server — validation must
	// return before any withConn attempt, so no connection is ever dialed.
	cfg := mustRegistrarConfig()
	adapter, err := epp.NewGenericEPPAdapter("test", cfg)
	require.NoError(t, err)

	_, err = adapter.CheckDomain(context.Background(), "bad_name.com")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr), "expected *EPPError, got %T: %v", err, err)
	assert.Equal(t, 2005, eppErr.Code)
}

func TestInfoDomain_InvalidName_ReturnsCode2005(t *testing.T) {
	cfg := mustRegistrarConfig()
	adapter, err := epp.NewGenericEPPAdapter("test", cfg)
	require.NoError(t, err)

	_, err = adapter.InfoDomain(context.Background(), "-leadinghyphen.com")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2005, eppErr.Code)
}

func TestCheckContact_InvalidHandle_ReturnsCode2005(t *testing.T) {
	cfg := mustRegistrarConfig()
	adapter, err := epp.NewGenericEPPAdapter("test", cfg)
	require.NoError(t, err)

	_, err = adapter.CheckContact(context.Background(), "bad handle with spaces")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2005, eppErr.Code)
}

func TestCheckHost_InvalidName_ReturnsCode2005(t *testing.T) {
	cfg := mustRegistrarConfig()
	adapter, err := epp.NewGenericEPPAdapter("test", cfg)
	require.NoError(t, err)

	_, err = adapter.CheckHost(context.Background(), "bad_host.example.com")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2005, eppErr.Code)
}

func TestCreateDomain_InvalidAuthInfo_ReturnsCode2005(t *testing.T) {
	cfg := mustRegistrarConfig()
	adapter, err := epp.NewGenericEPPAdapter("test", cfg)
	require.NoError(t, err)

	_, err = adapter.CreateDomain(context.Background(), registrar.DomainCreateRequest{
		Name:     "example.com",
		AuthInfo: "short", // only 5 chars — below minimum 6
	})
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2005, eppErr.Code)
}

// ---- IDN wire-level tests ----

// TestCheckDomain_IDN verifies that unicode domain names are sent as ACE (Punycode) on the wire.
// All four EPP providers embed GenericEPPAdapter, so this covers internetx/nicat/eurid/denic.
//
// The mock's srv.Received channel accumulates both the LOGIN frame (from scriptLogin) and the
// domain:check frame. We use drainAndAssert to skip the LOGIN frame and find the check frame.
// For the "must NOT contain" assertion, we read all frames up to a timeout and check none
// of them contain the raw unicode form.
func TestCheckDomain_IDN(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOnWire string
		wantAbsent string // must NOT appear in any wire frame (empty means skip check)
	}{
		{"unicode münchen.de", "münchen.de", "xn--mnchen-3ya", "münchen"},
		{"pure ascii example.com", "example.com", "example.com", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter, srv := newAdapter(t)
			scriptLogin(t, srv, adapter)

			srv.Expect <- domainCheckResponseXML

			results, err := adapter.CheckDomain(context.Background(), tc.input)
			require.NoError(t, err)
			require.NotEmpty(t, results)

			// drainAndAssert reads frames (skipping LOGIN) until it finds one containing
			// wantOnWire; it fails the test if no such frame arrives within 2 seconds.
			drainAndAssert(t, srv, tc.wantOnWire)

			if tc.wantAbsent != "" {
				// Collect any remaining frames and assert none contains the raw unicode form.
				deadline := time.After(200 * time.Millisecond)
			drain:
				for {
					select {
					case frame := <-srv.Received:
						assert.NotContains(t, string(frame), tc.wantAbsent,
							"raw unicode must NOT appear on the wire")
					case <-deadline:
						break drain
					}
				}
			}
			if tc.wantAbsent == "" {
				// For pure ASCII: confirm no Punycode prefix was introduced.
				// drainAndAssert already consumed the check frame; just verify nothing else arrived.
				select {
				case frame := <-srv.Received:
					assert.NotContains(t, string(frame), "xn--",
						"ASCII domain must not gain xn-- prefix")
				default:
					// No extra frames — expected.
				}
			}
		})
	}
}

// TestCreateContact_UTF8Postal verifies that contact postal fields with UTF-8
// characters (Müller, München) marshal to valid UTF-8 XML on the wire.
// All four EPP providers share GenericEPPAdapter and eppxml structs, so this
// single test covers internetx/nicat/eurid/denic postal marshalling.
//
// The mock accumulates both the LOGIN frame and the contact:create frame.
// We read frames until we find one containing "contact:create" (skipping LOGIN).
func TestCreateContact_UTF8Postal(t *testing.T) {
	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	srv.Expect <- successXML

	req := registrar.ContactCreateRequest{
		ID:          "MUELLER-1",
		Name:        "Müller",
		Org:         "Müller GmbH",
		Street:      []string{"Hauptstraße 1"},
		City:        "München",
		CountryCode: "DE",
		Email:       "m@example.de",
		AuthInfo:    "secret123",
	}

	result, err := adapter.CreateContact(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "MUELLER-1", result.ID)

	// Read frames until the contact:create frame arrives (LOGIN frame is skipped).
	var frame []byte
	deadline := time.After(2 * time.Second)
	for frame == nil {
		select {
		case f := <-srv.Received:
			if strings.Contains(string(f), "contact:create") || strings.Contains(string(f), "ContactCreate") || strings.Contains(string(f), "MUELLER-1") {
				frame = f
			}
		case <-deadline:
			t.Fatal("timeout waiting for contact:create frame")
		}
	}

	assert.True(t, utf8.Valid(frame), "contact:create frame must be valid UTF-8")
	assert.True(t, strings.Contains(string(frame), "Müller"), "frame must contain UTF-8 name Müller")
	assert.True(t, strings.Contains(string(frame), "München"), "frame must contain UTF-8 city München")
}
