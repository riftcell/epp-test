//go:build unit

package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/registrar"
)

// extendedStub extends stubRegistrar with recording fields for all operations
// needed by executor tests.
type extendedStub struct {
	stubRegistrar

	// domain ops
	updateDomainCalled bool
	updateDomainReq    registrar.DomainUpdateRequest
	deleteDomainCalled bool
	deleteDomainName   string
	renewDomainCalled  bool
	renewDomainName    string
	renewDomainYears   int
	renewDomainResult  registrar.DomainResult
	transferDomainCalled bool
	transferDomainReq    registrar.DomainTransferRequest

	// contact ops
	checkContactCalled bool
	checkContactIDs    []string
	infoContactCalled  bool
	infoContactID      string
	infoContactResult  registrar.ContactResult
	updateContactCalled bool
	updateContactReq    registrar.ContactUpdateRequest
	deleteContactCalled bool
	deleteContactID     string

	// host ops
	checkHostCalled  bool
	checkHostNames   []string
	createHostCalled bool
	createHostReq    registrar.HostCreateRequest
	infoHostCalled   bool
	infoHostName     string
	infoHostResult   registrar.HostResult
	updateHostCalled bool
	updateHostReq    registrar.HostUpdateRequest
	deleteHostCalled bool
	deleteHostName   string

	// poll ops
	pollReadCalled  bool
	pollReadResult  registrar.PollMessage
	pollAckCalled   bool
	pollAckMsgID    string

	// session ops
	loginCalled  bool
	logoutCalled bool
	pingCalled   bool
}

func (s *extendedStub) UpdateDomain(_ context.Context, req registrar.DomainUpdateRequest) (registrar.DomainResult, error) {
	s.updateDomainCalled = true
	s.updateDomainReq = req
	return registrar.DomainResult{Name: req.Name}, nil
}

func (s *extendedStub) DeleteDomain(_ context.Context, name string) error {
	s.deleteDomainCalled = true
	s.deleteDomainName = name
	return nil
}

func (s *extendedStub) RenewDomain(_ context.Context, name string, years int) (registrar.DomainResult, error) {
	s.renewDomainCalled = true
	s.renewDomainName = name
	s.renewDomainYears = years
	return s.renewDomainResult, nil
}

func (s *extendedStub) TransferDomain(_ context.Context, req registrar.DomainTransferRequest) (registrar.DomainResult, error) {
	s.transferDomainCalled = true
	s.transferDomainReq = req
	return registrar.DomainResult{Name: req.Name}, nil
}

func (s *extendedStub) CheckContact(_ context.Context, ids ...string) ([]registrar.ContactResult, error) {
	s.checkContactCalled = true
	s.checkContactIDs = ids
	results := make([]registrar.ContactResult, len(ids))
	for i, id := range ids {
		results[i] = registrar.ContactResult{ID: id}
	}
	return results, nil
}

func (s *extendedStub) InfoContact(_ context.Context, id string) (registrar.ContactResult, error) {
	s.infoContactCalled = true
	s.infoContactID = id
	return s.infoContactResult, nil
}

func (s *extendedStub) UpdateContact(_ context.Context, req registrar.ContactUpdateRequest) (registrar.ContactResult, error) {
	s.updateContactCalled = true
	s.updateContactReq = req
	return registrar.ContactResult{ID: req.ID}, nil
}

func (s *extendedStub) DeleteContact(_ context.Context, id string) error {
	s.deleteContactCalled = true
	s.deleteContactID = id
	return nil
}

func (s *extendedStub) CheckHost(_ context.Context, names ...string) ([]registrar.HostResult, error) {
	s.checkHostCalled = true
	s.checkHostNames = names
	results := make([]registrar.HostResult, len(names))
	for i, n := range names {
		results[i] = registrar.HostResult{Name: n}
	}
	return results, nil
}

func (s *extendedStub) CreateHost(_ context.Context, req registrar.HostCreateRequest) (registrar.HostResult, error) {
	s.createHostCalled = true
	s.createHostReq = req
	return registrar.HostResult{Name: req.Name}, nil
}

func (s *extendedStub) InfoHost(_ context.Context, name string) (registrar.HostResult, error) {
	s.infoHostCalled = true
	s.infoHostName = name
	return s.infoHostResult, nil
}

func (s *extendedStub) UpdateHost(_ context.Context, req registrar.HostUpdateRequest) (registrar.HostResult, error) {
	s.updateHostCalled = true
	s.updateHostReq = req
	return registrar.HostResult{Name: req.Name}, nil
}

func (s *extendedStub) DeleteHost(_ context.Context, name string) error {
	s.deleteHostCalled = true
	s.deleteHostName = name
	return nil
}

func (s *extendedStub) PollRead(_ context.Context) (registrar.PollMessage, error) {
	s.pollReadCalled = true
	return s.pollReadResult, nil
}

func (s *extendedStub) PollAck(_ context.Context, msgID string) error {
	s.pollAckCalled = true
	s.pollAckMsgID = msgID
	return nil
}

func (s *extendedStub) Login(_ context.Context) error {
	s.loginCalled = true
	return nil
}

func (s *extendedStub) Logout(_ context.Context) error {
	s.logoutCalled = true
	return nil
}

func (s *extendedStub) Ping(_ context.Context) error {
	s.pingCalled = true
	return nil
}

// compile-time check
var _ registrar.Registrar = (*extendedStub)(nil)

// ---- intParam tests ----

func TestIntParam_FromInt(t *testing.T) {
	got := intParam(map[string]any{"years": 3}, "years")
	assert.Equal(t, 3, got)
}

func TestIntParam_FromString(t *testing.T) {
	got := intParam(map[string]any{"years": "3"}, "years")
	assert.Equal(t, 3, got)
}

func TestIntParam_FromFloat64(t *testing.T) {
	got := intParam(map[string]any{"years": float64(5)}, "years")
	assert.Equal(t, 5, got)
}

func TestIntParam_FromInt64(t *testing.T) {
	got := intParam(map[string]any{"years": int64(7)}, "years")
	assert.Equal(t, 7, got)
}

func TestIntParam_MissingKey_ReturnsZero(t *testing.T) {
	got := intParam(map[string]any{}, "years")
	assert.Equal(t, 0, got)
}

func TestIntParam_UnknownType_ReturnsZero(t *testing.T) {
	got := intParam(map[string]any{"years": true}, "years")
	assert.Equal(t, 0, got)
}

// ---- execUpdateDomain ----

func TestExecUpdateDomain(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["update_domain"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"name": "d.at",
	})
	require.NoError(t, err)
	assert.True(t, stub.updateDomainCalled)
	assert.Equal(t, "d.at", stub.updateDomainReq.Name)

	dr, ok := result.(registrar.DomainResult)
	require.True(t, ok)
	assert.Equal(t, "d.at", dr.Name)
}

// ---- execDeleteDomain ----

func TestExecDeleteDomain(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["delete_domain"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{"name": "d.at"})
	require.NoError(t, err)
	assert.True(t, stub.deleteDomainCalled)
	assert.Equal(t, "d.at", stub.deleteDomainName)
}

// ---- execRenewDomain ----

func TestExecRenewDomain(t *testing.T) {
	stub := &extendedStub{
		renewDomainResult: registrar.DomainResult{Name: "d.at"},
	}

	fn := opTable["renew_domain"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"name":  "d.at",
		"years": "2",
	})
	require.NoError(t, err)
	assert.True(t, stub.renewDomainCalled)
	assert.Equal(t, "d.at", stub.renewDomainName)
	assert.Equal(t, 2, stub.renewDomainYears)

	dr, ok := result.(registrar.DomainResult)
	require.True(t, ok)
	assert.Equal(t, "d.at", dr.Name)
}

// ---- execTransferDomain ----

func TestExecTransferDomain(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["transfer_domain"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"name":      "d.at",
		"op":        "request",
		"auth_info": "secret",
	})
	require.NoError(t, err)
	assert.True(t, stub.transferDomainCalled)
	assert.Equal(t, "d.at", stub.transferDomainReq.Name)
	assert.Equal(t, "request", stub.transferDomainReq.Op)

	dr, ok := result.(registrar.DomainResult)
	require.True(t, ok)
	assert.Equal(t, "d.at", dr.Name)
}

// ---- execCheckContact ----

func TestExecCheckContact(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["check_contact"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"ids": []any{"JDOE-1", "JDOE-2"},
	})
	require.NoError(t, err)
	assert.True(t, stub.checkContactCalled)
	assert.Equal(t, []string{"JDOE-1", "JDOE-2"}, stub.checkContactIDs)

	results, ok := result.([]registrar.ContactResult)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

// ---- execCheckContact single string param ----

func TestExecCheckContact_SingleString(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["check_contact"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{
		"ids": "JDOE-1",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"JDOE-1"}, stub.checkContactIDs)
}

// ---- execInfoContact ----

func TestExecInfoContact(t *testing.T) {
	stub := &extendedStub{
		infoContactResult: registrar.ContactResult{ID: "JDOE-1", Name: "John Doe"},
	}

	fn := opTable["info_contact"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{"id": "JDOE-1"})
	require.NoError(t, err)
	assert.True(t, stub.infoContactCalled)
	assert.Equal(t, "JDOE-1", stub.infoContactID)

	cr, ok := result.(registrar.ContactResult)
	require.True(t, ok)
	assert.Equal(t, "JDOE-1", cr.ID)
}

// ---- execUpdateContact ----

func TestExecUpdateContact(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["update_contact"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{
		"id":    "JDOE-1",
		"email": "new@example.com",
	})
	require.NoError(t, err)
	assert.True(t, stub.updateContactCalled)
	assert.Equal(t, "JDOE-1", stub.updateContactReq.ID)
}

// ---- execDeleteContact ----

func TestExecDeleteContact(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["delete_contact"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{"id": "JDOE-1"})
	require.NoError(t, err)
	assert.True(t, stub.deleteContactCalled)
	assert.Equal(t, "JDOE-1", stub.deleteContactID)
}

// ---- execCheckHost ----

func TestExecCheckHost(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["check_host"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"names": []any{"ns1.example.com"},
	})
	require.NoError(t, err)
	assert.True(t, stub.checkHostCalled)
	assert.Equal(t, []string{"ns1.example.com"}, stub.checkHostNames)

	results, ok := result.([]registrar.HostResult)
	require.True(t, ok)
	assert.Len(t, results, 1)
}

// ---- execCreateHost ----

func TestExecCreateHost(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["create_host"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"name": "ns1.example.com",
	})
	require.NoError(t, err)
	assert.True(t, stub.createHostCalled)
	assert.Equal(t, "ns1.example.com", stub.createHostReq.Name)

	hr, ok := result.(registrar.HostResult)
	require.True(t, ok)
	assert.Equal(t, "ns1.example.com", hr.Name)
}

// ---- execInfoHost ----

func TestExecInfoHost(t *testing.T) {
	stub := &extendedStub{
		infoHostResult: registrar.HostResult{Name: "ns1.example.com"},
	}

	fn := opTable["info_host"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{"name": "ns1.example.com"})
	require.NoError(t, err)
	assert.True(t, stub.infoHostCalled)
	assert.Equal(t, "ns1.example.com", stub.infoHostName)

	hr, ok := result.(registrar.HostResult)
	require.True(t, ok)
	assert.Equal(t, "ns1.example.com", hr.Name)
}

// ---- execUpdateHost ----

func TestExecUpdateHost(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["update_host"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{
		"name": "ns1.example.com",
	})
	require.NoError(t, err)
	assert.True(t, stub.updateHostCalled)
	assert.Equal(t, "ns1.example.com", stub.updateHostReq.Name)
}

// ---- execDeleteHost ----

func TestExecDeleteHost(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["delete_host"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{"name": "ns1.example.com"})
	require.NoError(t, err)
	assert.True(t, stub.deleteHostCalled)
	assert.Equal(t, "ns1.example.com", stub.deleteHostName)
}

// ---- execPollRead ----

func TestExecPollRead_ReturnsMessage(t *testing.T) {
	stub := &extendedStub{
		pollReadResult: registrar.PollMessage{ID: "m1", Content: "Transfer requested"},
	}

	fn := opTable["poll"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{})
	require.NoError(t, err)
	assert.True(t, stub.pollReadCalled)

	msg, ok := result.(registrar.PollMessage)
	require.True(t, ok)
	assert.Equal(t, "m1", msg.ID)
}

// ---- execPollAck ----

func TestExecPollAck_UsesIDParam(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["poll_ack"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{"id": "msg-42"})
	require.NoError(t, err)
	assert.True(t, stub.pollAckCalled)
	assert.Equal(t, "msg-42", stub.pollAckMsgID)
}

func TestExecPollAck_UsesMsgIDParam(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["poll_ack"]
	require.NotNil(t, fn)

	// "msg_id" is the alternate key name.
	_, err := fn(context.Background(), stub, map[string]any{"msg_id": "msg-99"})
	require.NoError(t, err)
	assert.True(t, stub.pollAckCalled)
	assert.Equal(t, "msg-99", stub.pollAckMsgID)
}

// ---- execLogin ----

func TestExecLogin(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["login"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{})
	require.NoError(t, err)
	assert.True(t, stub.loginCalled)
}

// ---- execLogout ----

func TestExecLogout(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["logout"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{})
	require.NoError(t, err)
	assert.True(t, stub.logoutCalled)
}

// ---- execPing (via opTable["ping"]) ----

func TestExecPing(t *testing.T) {
	stub := &extendedStub{}

	fn := opTable["ping"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{})
	require.NoError(t, err)
	assert.True(t, stub.pingCalled)
}
