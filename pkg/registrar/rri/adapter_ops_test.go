//go:build unit

package rri_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rri_mock "github.com/riftcell/epp-test/pkg/mock/rri"
	"github.com/riftcell/epp-test/pkg/registrar"
)

// loginAndDrain is a helper that logs in and drains the LOGIN frame from Received.
func loginAndDrain(t *testing.T, srv *rri_mock.MockRRIServer, adapter interface {
	Login(context.Context) error
}) {
	t.Helper()
	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login precondition")
	drainReceived(srv)
}

// ---- Name ----

func TestRRIName(t *testing.T) {
	_, adapter := newTestAdapter(t)
	assert.Equal(t, "denic", adapter.Name())
}

// ---- Logout ----

func TestRRILogout_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	// Script LOGOUT response.
	srv.Expect <- []byte("RESULT: success\nSTID: mock-logout\n")

	err := adapter.Logout(context.Background())
	require.NoError(t, err)
}

// ---- Ping ----

func TestRRIPing_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	// Ping uses QUEUE-READ internally; script a success response.
	srv.Expect <- []byte("RESULT: success\nSTID: mock-ping\n")

	err := adapter.Ping(context.Background())
	require.NoError(t, err)
}

// ---- InfoDomain ----

func TestRRIInfoDomain_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-info\nHOLDER: DENIC-1000-JDOE\n")

	result, err := adapter.InfoDomain(context.Background(), "example.de")
	require.NoError(t, err)
	assert.Equal(t, "example.de", result.Name)
	assert.Equal(t, "DENIC-1000-JDOE", result.Registrant)
}

// ---- UpdateDomain ----

func TestRRIUpdateDomain_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-update\n")

	req := registrar.DomainUpdateRequest{
		Name:           "example.de",
		AddNameservers: []string{"ns1.example.de"},
	}
	result, err := adapter.UpdateDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.de", result.Name)
}

// ---- DeleteDomain ----

func TestRRIDeleteDomain_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-delete\n")

	err := adapter.DeleteDomain(context.Background(), "example.de")
	require.NoError(t, err)
}

// ---- RenewDomain returns 2101 ----

func TestRRIRenewDomain_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	_, err = adapter.RenewDomain(context.Background(), "example.de", 1)
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- TransferDomain authinfo1 ----

func TestRRITransferDomain_AuthInfo1(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-authinfo1\n")

	req := registrar.DomainTransferRequest{
		Name:     "example.de",
		Op:       "authinfo1",
		AuthInfo: "secret-code",
	}
	result, err := adapter.TransferDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.de", result.Name)

	// Verify the frame sent was CREATE-AUTHINFO1.
	var frame []byte
	select {
	case frame = <-srv.Received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for frame")
	}
	fields := rri_mock.ParseKV(string(frame))
	action := fields["action"]
	require.Len(t, action, 1)
	assert.Equal(t, "CREATE-AUTHINFO1", action[0])
}

// ---- TransferDomain authinfo2 ----

func TestRRITransferDomain_AuthInfo2(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-authinfo2\n")

	req := registrar.DomainTransferRequest{
		Name: "example.de",
		Op:   "authinfo2",
	}
	result, err := adapter.TransferDomain(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "example.de", result.Name)

	var frame []byte
	select {
	case frame = <-srv.Received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for frame")
	}
	fields := rri_mock.ParseKV(string(frame))
	action := fields["action"]
	require.Len(t, action, 1)
	assert.Equal(t, "CREATE-AUTHINFO2", action[0])
}

// ---- TransferDomain unknown op returns 2101 ----

func TestRRITransferDomain_UnknownOp_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	req := registrar.DomainTransferRequest{
		Name: "example.de",
		Op:   "reject", // not implemented in RRI
	}
	_, err = adapter.TransferDomain(context.Background(), req)
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- CheckContact ----

func TestRRICheckContact_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	// success response means the handle exists
	srv.Expect <- []byte("RESULT: success\nSTID: mock-checkcontact\n")

	results, err := adapter.CheckContact(context.Background(), "DENIC-1000-JDOE")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "DENIC-1000-JDOE", results[0].ID)
}

// ---- InfoContact ----

func TestRRIInfoContact_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-infocontact\nNAME: John Doe\nORGANISATION: Example GmbH\nEMAIL: john@example.de\n")

	result, err := adapter.InfoContact(context.Background(), "DENIC-1000-JDOE")
	require.NoError(t, err)
	assert.Equal(t, "John Doe", result.Name)
	assert.Equal(t, "john@example.de", result.Email)
}

// ---- CreateContact ----

func TestRRICreateContact_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-createcontact\n")

	req := registrar.ContactCreateRequest{
		ID:          "DENIC-1000-JDOE",
		Name:        "John Doe",
		Email:       "john@example.de",
		City:        "Berlin",
		CountryCode: "DE",
	}
	result, err := adapter.CreateContact(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "DENIC-1000-JDOE", result.ID)
}

// ---- UpdateContact returns 2101 ----

func TestRRIUpdateContact_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	req := registrar.ContactUpdateRequest{ID: "DENIC-1000-JDOE", Email: "new@example.de"}
	_, err = adapter.UpdateContact(context.Background(), req)
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- DeleteContact returns 2101 ----

func TestRRIDeleteContact_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	err = adapter.DeleteContact(context.Background(), "DENIC-1000-JDOE")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- CheckHost returns 2101 ----

func TestRRICheckHost_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	_, err = adapter.CheckHost(context.Background(), "ns1.example.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- InfoHost returns 2101 ----

func TestRRIInfoHost_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	_, err = adapter.InfoHost(context.Background(), "ns1.example.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- UpdateHost returns 2101 ----

func TestRRIUpdateHost_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	req := registrar.HostUpdateRequest{Name: "ns1.example.de"}
	_, err = adapter.UpdateHost(context.Background(), req)
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- DeleteHost returns 2101 ----

func TestRRIDeleteHost_Returns2101(t *testing.T) {
	_, adapter := newTestAdapter(t)

	err := adapter.Login(context.Background())
	require.NoError(t, err)

	err = adapter.DeleteHost(context.Background(), "ns1.example.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2101, eppErr.Code)
}

// ---- PollRead WithMessage ----

func TestRRIPollRead_WithMessage(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	// INFO field must be in the format "<numeric_id> <message_text>" per go-rriclient parsing.
	srv.Expect <- []byte("RESULT: success\nSTID: mock-poll\nMSGID: poll-001\nMSGTYPE: domainCreate\nINFO: 12345678 example.de created\n")

	msg, err := adapter.PollRead(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "poll-001", msg.ID)
	assert.Equal(t, "domainCreate", msg.Type)
	assert.NotEmpty(t, msg.Content)
}

// ---- PollAck ----

func TestRRIPollAck_Success(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: success\nSTID: mock-pollack\n")

	err := adapter.PollAck(context.Background(), "poll-001")
	require.NoError(t, err)
}

// ---- mapRRICodeToEPP tested indirectly via CheckDomain errors ----

// TestRRIMapCode_NotFound verifies that RRI business code 83000000001 maps to EPP 2303.
func TestRRIMapCode_NotFound(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: failed\nSTID: s\nERROR: 83000000001 Not found\n")

	_, err := adapter.CheckDomain(context.Background(), "notfound.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2303, eppErr.Code)
}

// TestRRIMapCode_AlreadyExists verifies that RRI business code 83000000002 maps to EPP 2302.
func TestRRIMapCode_AlreadyExists(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: failed\nSTID: s\nERROR: 83000000002 Already exists\n")

	_, err := adapter.CheckDomain(context.Background(), "taken.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2302, eppErr.Code)
}

// TestRRIMapCode_UnmappedCode verifies that unmapped RRI codes map to EPP 2400.
func TestRRIMapCode_UnmappedCode(t *testing.T) {
	srv, adapter := newTestAdapter(t)

	loginAndDrain(t, srv, adapter)

	srv.Expect <- []byte("RESULT: failed\nSTID: s\nERROR: 99999999 Unknown error\n")

	_, err := adapter.CheckDomain(context.Background(), "unknown.de")
	require.Error(t, err)

	var eppErr *registrar.EPPError
	require.True(t, errors.As(err, &eppErr))
	assert.Equal(t, 2400, eppErr.Code)
}

// ---- IDN wire-level tests ----

// TestRRICheckDomain_IDN verifies that unicode domain names are normalized to ACE
// (Punycode) form before being sent over the wire in DENIC RRI.
//
// go-rriclient sends both "domain:" (unicode) and "domain-ace:" (Punycode) fields per
// the RRI protocol; we assert the "domain-ace" field carries the ACE form and that the
// adapter returns the ACE name in DomainResult.Name.
func TestRRICheckDomain_IDN(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantWireDomain string // expected value of the "domain-ace" wire field
	}{
		{"unicode münchen.de", "münchen.de", "xn--mnchen-3ya.de"},
		{"pure ascii example.de", "example.de", "example.de"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, adapter := newTestAdapter(t)
			loginAndDrain(t, srv, adapter)

			srv.Expect <- []byte("RESULT: success\nSTID: mock-idn\n")

			results, err := adapter.CheckDomain(context.Background(), tc.input)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, tc.wantWireDomain, results[0].Name,
				"adapter must return ACE name in DomainResult")

			var frame []byte
			select {
			case frame = <-srv.Received:
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for domain:check frame")
			}

			// go-rriclient transmits both "domain:" (unicode) and "domain-ace:" (Punycode).
			// Assert that "domain-ace" carries the ACE-normalized form.
			fields := rri_mock.ParseKV(string(frame))
			require.NotEmpty(t, fields["domain-ace"], "domain-ace field must be present in wire frame")
			assert.Equal(t, tc.wantWireDomain, fields["domain-ace"][0],
				"domain-ace field must carry the ACE-normalized form on the wire")
		})
	}
}
