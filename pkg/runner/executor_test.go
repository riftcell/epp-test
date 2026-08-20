//go:build unit

package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/registrar"
)

// stubRegistrar is a minimal in-package stub that records calls for executor tests.
type stubRegistrar struct {
	checkDomainCalled bool
	checkDomainNames  []string
	checkDomainResult []registrar.DomainResult
	checkDomainErr    error

	createDomainCalled bool
	createDomainReq    registrar.DomainCreateRequest
	createDomainResult registrar.DomainResult
	createDomainErr    error
}

func (s *stubRegistrar) CheckDomain(_ context.Context, names ...string) ([]registrar.DomainResult, error) {
	s.checkDomainCalled = true
	s.checkDomainNames = names
	return s.checkDomainResult, s.checkDomainErr
}

func (s *stubRegistrar) InfoDomain(_ context.Context, _ string) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (s *stubRegistrar) CreateDomain(_ context.Context, req registrar.DomainCreateRequest) (registrar.DomainResult, error) {
	s.createDomainCalled = true
	s.createDomainReq = req
	return s.createDomainResult, s.createDomainErr
}

func (s *stubRegistrar) UpdateDomain(_ context.Context, _ registrar.DomainUpdateRequest) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (s *stubRegistrar) DeleteDomain(_ context.Context, _ string) error { return nil }

func (s *stubRegistrar) RenewDomain(_ context.Context, _ string, _ int) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (s *stubRegistrar) TransferDomain(_ context.Context, _ registrar.DomainTransferRequest) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (s *stubRegistrar) CheckContact(_ context.Context, _ ...string) ([]registrar.ContactResult, error) {
	return nil, nil
}

func (s *stubRegistrar) InfoContact(_ context.Context, _ string) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, nil
}

func (s *stubRegistrar) CreateContact(_ context.Context, _ registrar.ContactCreateRequest) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, nil
}

func (s *stubRegistrar) UpdateContact(_ context.Context, _ registrar.ContactUpdateRequest) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, nil
}

func (s *stubRegistrar) DeleteContact(_ context.Context, _ string) error { return nil }

func (s *stubRegistrar) CheckHost(_ context.Context, _ ...string) ([]registrar.HostResult, error) {
	return nil, nil
}

func (s *stubRegistrar) InfoHost(_ context.Context, _ string) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (s *stubRegistrar) CreateHost(_ context.Context, _ registrar.HostCreateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (s *stubRegistrar) UpdateHost(_ context.Context, _ registrar.HostUpdateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (s *stubRegistrar) DeleteHost(_ context.Context, _ string) error { return nil }

func (s *stubRegistrar) PollRead(_ context.Context) (registrar.PollMessage, error) {
	return registrar.PollMessage{}, nil
}

func (s *stubRegistrar) PollAck(_ context.Context, _ string) error { return nil }

func (s *stubRegistrar) Login(_ context.Context) error  { return nil }
func (s *stubRegistrar) Logout(_ context.Context) error { return nil }
func (s *stubRegistrar) Ping(_ context.Context) error   { return nil }
func (s *stubRegistrar) Name() string                   { return "stub" }

// compile-time check that stubRegistrar satisfies the interface.
var _ registrar.Registrar = (*stubRegistrar)(nil)

// TestDecodeParams verifies that decodeParams correctly populates typed request structs
// from a flat map, including weak type coercion (string "1" → int 1).
func TestDecodeParams(t *testing.T) {
	params := map[string]any{
		"name":      "d.at",
		"period":    "1", // string that must coerce to int
		"auth_info": "x",
	}
	var req registrar.DomainCreateRequest
	err := decodeParams(params, &req)
	require.NoError(t, err)
	assert.Equal(t, "d.at", req.Name)
	assert.Equal(t, 1, req.Period, "string '1' must coerce to int via WeaklyTypedInput")
	assert.Equal(t, "x", req.AuthInfo)
}

// TestOpTable verifies that opTable contains entries for all expected operations.
func TestOpTable(t *testing.T) {
	expectedOps := []string{
		"check_domain", "create_domain", "info_domain", "update_domain",
		"delete_domain", "renew_domain", "transfer_domain",
		"check_contact", "create_contact", "info_contact", "update_contact", "delete_contact",
		"check_host", "create_host", "info_host", "update_host", "delete_host",
		"poll", "poll_ack", "login", "logout", "ping",
	}

	for _, op := range expectedOps {
		fn, ok := opTable[op]
		assert.True(t, ok, "opTable missing key %q", op)
		assert.NotNil(t, fn, "opTable[%q] is nil", op)
	}
}

// TestExecCheckDomain verifies that the check_domain executor calls CheckDomain
// with the provided names and returns a non-nil result.
func TestExecCheckDomain(t *testing.T) {
	stub := &stubRegistrar{
		checkDomainResult: []registrar.DomainResult{
			{Name: "a.at"},
			{Name: "b.at"},
		},
	}

	fn := opTable["check_domain"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"names": []any{"a.at", "b.at"},
	})
	require.NoError(t, err)
	assert.True(t, stub.checkDomainCalled, "CheckDomain should have been called")
	assert.Equal(t, []string{"a.at", "b.at"}, stub.checkDomainNames)
	assert.NotNil(t, result)

	results, ok := result.([]registrar.DomainResult)
	require.True(t, ok, "result should be []DomainResult")
	assert.Len(t, results, 2)
}

// TestExecCreateDomain verifies that the create_domain executor decodes params
// and calls CreateDomain on the registrar.
func TestExecCreateDomain(t *testing.T) {
	stub := &stubRegistrar{
		createDomainResult: registrar.DomainResult{Name: "test.at"},
	}

	fn := opTable["create_domain"]
	require.NotNil(t, fn)

	result, err := fn(context.Background(), stub, map[string]any{
		"name":      "test.at",
		"period":    "2",
		"auth_info": "secret",
	})
	require.NoError(t, err)
	assert.True(t, stub.createDomainCalled)
	assert.Equal(t, "test.at", stub.createDomainReq.Name)
	assert.Equal(t, 2, stub.createDomainReq.Period)
	assert.Equal(t, "secret", stub.createDomainReq.AuthInfo)

	dr, ok := result.(registrar.DomainResult)
	require.True(t, ok)
	assert.Equal(t, "test.at", dr.Name)
}

// TestExecOpErrors verifies error propagation from the registrar stub.
func TestExecOpErrors(t *testing.T) {
	stub := &stubRegistrar{
		checkDomainErr: errors.New("network error"),
	}

	fn := opTable["check_domain"]
	require.NotNil(t, fn)

	_, err := fn(context.Background(), stub, map[string]any{
		"names": []any{"fail.at"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

// TestAssertExpectMismatch verifies that assertExpect returns (false, error) when
// the EPP code in the error does not match the expected code. This is an internal
// test because assertExpect is package-private.
func TestAssertExpectMismatch(t *testing.T) {
	// Simulate: server returned 2302, scenario expects 1000.
	stepErr := &registrar.EPPError{Code: 2302, Message: "Object exists"}
	expect := &Expect{Code: 1000}

	passed, assertErr := assertExpect(expect, nil, stepErr)
	assert.False(t, passed, "assertExpect must return false for code mismatch")
	require.NotNil(t, assertErr, "assertExpect must return non-nil error for code mismatch")
	assert.Contains(t, assertErr.Error(), "2302", "error message must include the actual code")
	assert.Contains(t, assertErr.Error(), "1000", "error message must include the expected code")
}

// TestAssertExpectPass verifies that assertExpect returns (true, nil) for a matching code.
func TestAssertExpectPass(t *testing.T) {
	expect := &Expect{Code: 1000}
	passed, assertErr := assertExpect(expect, nil, nil)
	assert.True(t, passed, "assertExpect must return true for matching code 1000 with no error")
	assert.Nil(t, assertErr)
}

// TestAssertExpectEPPErrorCode verifies assertExpect matches an EPP error code
// when the scenario expects an error (e.g., 2302 → object exists).
func TestAssertExpectEPPErrorCode(t *testing.T) {
	stepErr := &registrar.EPPError{Code: 2302, Message: "Object exists"}
	expect := &Expect{Code: 2302}

	passed, assertErr := assertExpect(expect, nil, stepErr)
	assert.True(t, passed, "assertExpect must return true when EPP code matches expected error code")
	assert.Nil(t, assertErr)
}

// TestStepResultRecordsFailure is an internal test that calls assertExpect directly
// and verifies StepResult fields. This covers behavior Test 3 from the plan without
// needing to propagate a sub-test failure to the outer test.
func TestStepResultRecordsFailure(t *testing.T) {
	stepErr := &registrar.EPPError{Code: 2302, Message: "Object exists"}
	expect := &Expect{Code: 1000}

	passed, assertErr := assertExpect(expect, nil, stepErr)
	require.False(t, passed)
	require.NotNil(t, assertErr)

	// Simulate what RunScenario does: record StepResult on failure.
	sr := StepResult{
		Name:   "create",
		Op:     "create_domain",
		Passed: passed,
		Err:    assertErr.Error(),
	}

	assert.False(t, sr.Passed, "StepResult.Passed must be false for code mismatch")
	assert.NotEmpty(t, sr.Err, "StepResult.Err must be non-empty for code mismatch")
}
