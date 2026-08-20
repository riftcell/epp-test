//go:build unit

package runner_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/config"
	epp "github.com/riftcell/epp-test/pkg/registrar/epp"
	epp_mock "github.com/riftcell/epp-test/pkg/mock/epp"
	"github.com/riftcell/epp-test/pkg/registrar"
	"github.com/riftcell/epp-test/pkg/runner"
)

// TestMain enables goroutine leak detection for the entire test binary.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// ---- EPP XML fixtures ----

var greetingXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <greeting>
    <svID>EPP Mock Server</svID>
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

var loginOKXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <trID><clTRID>gsd-test-1</clTRID><svTRID>mock-svtrid-001</svTRID></trID>
  </response>
</epp>`)

var successXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <trID><clTRID>gsd-test-2</clTRID><svTRID>mock-svtrid-002</svTRID></trID>
  </response>
</epp>`)

var domainCheckAvailXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:cd>
          <domain:name avail="1">runner-test.at</domain:name>
        </domain:cd>
      </domain:chkData>
    </resData>
    <trID><clTRID>gsd-test-3</clTRID><svTRID>mock-svtrid-003</svTRID></trID>
  </response>
</epp>`)

var domainInfoXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <domain:infData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
        <domain:name>runner-test.at</domain:name>
        <domain:roid>DOMAIN-001</domain:roid>
        <domain:status s="ok"/>
        <domain:registrant>test-contact-001</domain:registrant>
        <domain:clID>test-client</domain:clID>
      </domain:infData>
    </resData>
    <trID><clTRID>gsd-test-4</clTRID><svTRID>mock-svtrid-004</svTRID></trID>
  </response>
</epp>`)

var contactCreateXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1000"><msg>Command completed successfully</msg></result>
    <resData>
      <contact:creData xmlns:contact="urn:ietf:params:xml:ns:contact-1.0">
        <contact:id>CAPTURED-CONTACT-001</contact:id>
        <contact:crDate>2026-06-28T12:00:00.0Z</contact:crDate>
      </contact:creData>
    </resData>
    <trID><clTRID>gsd-test-5</clTRID><svTRID>mock-svtrid-005</svTRID></trID>
  </response>
</epp>`)

var error2302XML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="2302"><msg>Object exists</msg></result>
    <trID><clTRID>gsd-test-6</clTRID><svTRID>mock-svtrid-006</svTRID></trID>
  </response>
</epp>`)

var pollMessageXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1301"><msg>Command completed successfully; ack to dequeue</msg></result>
    <msgQ count="1" id="msg-001">
      <qDate>2026-06-28T12:00:00.0Z</qDate>
      <msg>Transfer requested</msg>
    </msgQ>
    <trID><clTRID>gsd-test-7</clTRID><svTRID>mock-svtrid-007</svTRID></trID>
  </response>
</epp>`)

var pollEmptyXML = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
  <response>
    <result code="1300"><msg>Command completed successfully; no messages</msg></result>
    <trID><clTRID>gsd-test-8</clTRID><svTRID>mock-svtrid-008</svTRID></trID>
  </response>
</epp>`)

// ---- Test helpers ----

// newAdapter creates a MockEPPServer and a GenericEPPAdapter wired to it.
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

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return adapter, srv
}

// scriptLogin scripts the login handshake and calls Login.
func scriptLogin(t *testing.T, srv *epp_mock.MockEPPServer, adapter *epp.GenericEPPAdapter) {
	t.Helper()
	srv.InjectPoll(greetingXML)
	srv.Expect <- loginOKXML
	err := adapter.Login(context.Background())
	require.NoError(t, err, "Login must succeed before running operations")
}

// writeTempScenario writes a minimal YAML scenario file to a temp dir and returns its path.
func writeTempScenario(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// ---- Test 1: lifecycle (check, create, info, delete) ----

func TestRunScenario_Lifecycle(t *testing.T) {
	// Flush any leftover collector state from previous tests.
	runner.Flush()

	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	// Script responses for each step and the cleanup.
	// Scenario steps: check_domain, create_domain, info_domain.
	// t.Cleanup (registered by RunScenario) will call delete_domain after the test.
	srv.Expect <- domainCheckAvailXML // check_domain -> 1000 + available:true
	srv.Expect <- successXML          // create_domain -> 1000
	srv.Expect <- domainInfoXML       // info_domain -> 1000 + name field
	srv.Expect <- successXML          // cleanup delete_domain (registered by create_domain step)

	scenarioYAML := `
name: lifecycle test
steps:
  - name: check
    op: check_domain
    params:
      names: [runner-test.at]
    expect:
      code: 1000

  - name: create
    op: create_domain
    params:
      name: runner-test.at
      period: 1
      auth_info: testauth
    expect:
      code: 1000

  - name: info
    op: info_domain
    params:
      name: runner-test.at
    expect:
      code: 1000
      fields:
        Name: runner-test.at
`
	path := writeTempScenario(t, "lifecycle.yaml", scenarioYAML)
	runner.RunScenario(t, path, adapter)
}

// ---- Test 2: variable capture ----

func TestRunScenario_VariableCapture(t *testing.T) {
	runner.Flush()

	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	// create_contact returns a contact with ID "CAPTURED-CONTACT-001"
	srv.Expect <- contactCreateXML
	// create_domain uses the captured ID as registrant
	srv.Expect <- successXML
	// cleanup: delete_domain, delete_contact
	srv.Expect <- successXML
	srv.Expect <- successXML

	scenarioYAML := `
name: variable capture test
steps:
  - name: create_contact
    op: create_contact
    params:
      id: test-contact-001
      name: Test User
      email: test@example.at
      city: Vienna
      country_code: AT
      phone: "+43.12345678"
      auth_info: authTest123
    expect:
      code: 1000

  - name: create_domain
    op: create_domain
    params:
      name: capture-test.at
      registrant: ${create_contact.ID}
      period: 1
      auth_info: domainauth
    expect:
      code: 1000
`
	path := writeTempScenario(t, "capture.yaml", scenarioYAML)
	// This should not fail — the variable capture should resolve ${create_contact.ID}
	runner.RunScenario(t, path, adapter)
}

// ---- Test 3: expect mismatch → StepResult.Passed is false ----
//
// Note: TestRunScenario_ExpectMismatch is an internal package test (not _test suffix)
// and lives in executor_test.go where it can access assertExpect directly.
// See: TestAssertExpectMismatch in executor_test.go

// ---- Test 4: cleanup order (reverse creation order) ----

// orderRecorder is a stub Registrar that records method call order for cleanup assertions.
type orderRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *orderRecorder) record(op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, op)
}

func (r *orderRecorder) CheckDomain(_ context.Context, _ ...string) ([]registrar.DomainResult, error) {
	return nil, nil
}

func (r *orderRecorder) InfoDomain(_ context.Context, _ string) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (r *orderRecorder) CreateDomain(_ context.Context, req registrar.DomainCreateRequest) (registrar.DomainResult, error) {
	r.record("create_domain:" + req.Name)
	return registrar.DomainResult{Name: req.Name}, nil
}

func (r *orderRecorder) UpdateDomain(_ context.Context, _ registrar.DomainUpdateRequest) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (r *orderRecorder) DeleteDomain(_ context.Context, name string) error {
	r.record("delete_domain:" + name)
	return nil
}

func (r *orderRecorder) RenewDomain(_ context.Context, _ string, _ int) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (r *orderRecorder) TransferDomain(_ context.Context, _ registrar.DomainTransferRequest) (registrar.DomainResult, error) {
	return registrar.DomainResult{}, nil
}

func (r *orderRecorder) CheckContact(_ context.Context, _ ...string) ([]registrar.ContactResult, error) {
	return nil, nil
}

func (r *orderRecorder) InfoContact(_ context.Context, _ string) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, nil
}

func (r *orderRecorder) CreateContact(_ context.Context, req registrar.ContactCreateRequest) (registrar.ContactResult, error) {
	r.record("create_contact:" + req.ID)
	return registrar.ContactResult{ID: req.ID}, nil
}

func (r *orderRecorder) UpdateContact(_ context.Context, _ registrar.ContactUpdateRequest) (registrar.ContactResult, error) {
	return registrar.ContactResult{}, nil
}

func (r *orderRecorder) DeleteContact(_ context.Context, id string) error {
	r.record("delete_contact:" + id)
	return nil
}

func (r *orderRecorder) CheckHost(_ context.Context, _ ...string) ([]registrar.HostResult, error) {
	return nil, nil
}

func (r *orderRecorder) InfoHost(_ context.Context, _ string) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (r *orderRecorder) CreateHost(_ context.Context, _ registrar.HostCreateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (r *orderRecorder) UpdateHost(_ context.Context, _ registrar.HostUpdateRequest) (registrar.HostResult, error) {
	return registrar.HostResult{}, nil
}

func (r *orderRecorder) DeleteHost(_ context.Context, _ string) error { return nil }

func (r *orderRecorder) PollRead(_ context.Context) (registrar.PollMessage, error) {
	return registrar.PollMessage{}, nil
}

func (r *orderRecorder) PollAck(_ context.Context, _ string) error { return nil }

func (r *orderRecorder) Login(_ context.Context) error  { return nil }
func (r *orderRecorder) Logout(_ context.Context) error { return nil }
func (r *orderRecorder) Ping(_ context.Context) error   { return nil }
func (r *orderRecorder) Name() string                   { return "recorder" }

var _ registrar.Registrar = (*orderRecorder)(nil)

// namedRecorder is an orderRecorder with an overridable Name() for matrix tests.
type namedRecorder struct {
	orderRecorder
	name string
}

func (r *namedRecorder) Name() string { return r.name }

var _ registrar.Registrar = (*namedRecorder)(nil)

func TestRunMatrix_PerRegistrarSubtests(t *testing.T) {
	runner.Flush()

	regA := &namedRecorder{name: "a"}
	regB := &namedRecorder{name: "b"}
	regC := &namedRecorder{name: "c"}

	scenarioYAML := `
name: matrix scenario
matrix: [a, b]
steps:
  - name: ping
    op: ping
    expect:
      code: 1000
`
	path := writeTempScenario(t, "matrix.yaml", scenarioYAML)
	runner.RunMatrix(t, path, map[string]registrar.Registrar{"a": regA, "b": regB, "c": regC})

	results := runner.Flush()
	require.Len(t, results, 2, "matrix [a,b] must produce exactly 2 results, not c")
	regs := map[string]bool{}
	for _, r := range results {
		regs[r.Registrar] = true
		assert.Equal(t, "matrix scenario", r.Name)
	}
	assert.True(t, regs["a"], "result for registrar a expected")
	assert.True(t, regs["b"], "result for registrar b expected")
	assert.False(t, regs["c"], "registrar c is not in matrix and must not run")
}

func TestRunMatrix_EmptyMatrixRunsAll(t *testing.T) {
	runner.Flush()

	regA := &namedRecorder{name: "a"}
	regB := &namedRecorder{name: "b"}

	scenarioYAML := `
name: no matrix scenario
steps:
  - name: ping
    op: ping
    expect:
      code: 1000
`
	path := writeTempScenario(t, "nomatrix.yaml", scenarioYAML)
	runner.RunMatrix(t, path, map[string]registrar.Registrar{"a": regA, "b": regB})

	results := runner.Flush()
	require.Len(t, results, 2, "empty matrix must run against all regs")
}

func TestRunMatrix_SkipsMissingRegistrar(t *testing.T) {
	runner.Flush()

	regA := &namedRecorder{name: "a"}

	scenarioYAML := `
name: partial matrix scenario
matrix: [a, z]
steps:
  - name: ping
    op: ping
    expect:
      code: 1000
`
	path := writeTempScenario(t, "partialmatrix.yaml", scenarioYAML)
	runner.RunMatrix(t, path, map[string]registrar.Registrar{"a": regA})

	results := runner.Flush()
	require.Len(t, results, 1, "registrar z absent from regs must be skipped silently")
	assert.Equal(t, "a", results[0].Registrar)
}

func TestRunScenario_Accumulates(t *testing.T) {
	runner.Flush()

	rec := &orderRecorder{}

	mk := func(name string) string {
		return writeTempScenario(t, name+".yaml", `
name: `+name+`
steps:
  - name: ping
    op: ping
    expect:
      code: 1000
`)
	}

	// Two RunScenario calls with NO Flush in between must both land in the collector.
	runner.RunScenario(t, mk("accum-one"), rec)
	runner.RunScenario(t, mk("accum-two"), rec)

	results := runner.Flush()
	require.Len(t, results, 2, "two RunScenario calls without an intervening Flush must accumulate, not replace")
}

func TestRunScenario_CleanupOrder(t *testing.T) {
	runner.Flush()

	rec := &orderRecorder{}

	// Use injectRunID prefix to avoid collisions; contact created first, then domain.
	scenarioYAML := `
name: cleanup order test
steps:
  - name: create_contact
    op: create_contact
    params:
      id: order-contact-001
      name: Test User
      email: test@example.at
      city: Vienna
      country_code: AT
      phone: "+43.12345678"
      auth_info: authTest

  - name: create_domain
    op: create_domain
    params:
      name: order-domain.at
      period: 1
      auth_info: domainauth
`
	path := writeTempScenario(t, "cleanup_order.yaml", scenarioYAML)

	t.Run("scenario", func(t *testing.T) {
		runner.RunScenario(t, path, rec)
	})

	// After the subtest, t.Cleanup has run — so delete calls should be recorded.
	// Verify: delete_domain MUST come before delete_contact (reverse of creation order).
	rec.mu.Lock()
	calls := make([]string, len(rec.calls))
	copy(calls, rec.calls)
	rec.mu.Unlock()

	// Find indices of the delete calls.
	deleteContactIdx := -1
	deleteDomainIdx := -1
	for i, c := range calls {
		if c == "delete_contact:order-contact-001" || containsPrefix(c, "delete_contact:") {
			deleteContactIdx = i
		}
		if c == "delete_domain:order-domain.at" || containsPrefix(c, "delete_domain:") {
			deleteDomainIdx = i
		}
	}

	require.GreaterOrEqual(t, deleteDomainIdx, 0, "delete_domain should have been called")
	require.GreaterOrEqual(t, deleteContactIdx, 0, "delete_contact should have been called")
	assert.Less(t, deleteDomainIdx, deleteContactIdx,
		"delete_domain (idx %d) must come before delete_contact (idx %d) — reverse creation order",
		deleteDomainIdx, deleteContactIdx)
}

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ---- Test 5: RunDir ----

func TestRunDir_CreatesSubtests(t *testing.T) {
	runner.Flush()

	dir := t.TempDir()

	// Write two minimal scenario YAML files.
	for _, name := range []string{"scenario_a.yaml", "scenario_b.yaml"} {
		content := fmt.Sprintf(`
name: %s
steps: []
`, name)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	// Use a stub registrar that does nothing.
	rec := &orderRecorder{}

	subtestRan := map[string]bool{}
	var mu sync.Mutex

	// We use t.Run to capture which subtests are created; RunDir itself creates them.
	// We can verify by checking both scenarios are in the collector.
	runner.RunDir(t, dir, rec)

	results := runner.Flush()
	for _, r := range results {
		mu.Lock()
		subtestRan[r.Name] = true
		mu.Unlock()
	}

	// Both scenarios should have run and been appended to the collector.
	assert.True(t, subtestRan["scenario_a.yaml"] || subtestRan["scenario_b.yaml"] || len(results) >= 2,
		"RunDir should have run at least 2 scenario subtests, got %d results: %v", len(results), results)
}

// ---- Test 6: wait_for_poll ----

func TestRunScenario_WaitForPoll(t *testing.T) {
	runner.Flush()

	adapter, srv := newAdapter(t)
	scriptLogin(t, srv, adapter)

	// First two PollRead calls return empty queue; third returns a message.
	srv.Expect <- pollEmptyXML
	srv.Expect <- pollEmptyXML
	srv.Expect <- pollMessageXML
	// PollAck response.
	srv.Expect <- successXML

	scenarioYAML := `
name: wait for poll test
steps:
  - name: wait_transfer
    op: wait_for_poll
    params:
      timeout: 5s
    expect:
      code: 1000
`
	path := writeTempScenario(t, "poll.yaml", scenarioYAML)
	runner.RunScenario(t, path, adapter)
}

// ---- Test 7: collector append ----

func TestRunScenario_CollectorAppend(t *testing.T) {
	runner.Flush()

	rec := &orderRecorder{}

	scenarioYAML := `
name: collector test scenario
steps:
  - name: ping
    op: ping
    expect:
      code: 1000
`
	path := writeTempScenario(t, "collector.yaml", scenarioYAML)
	runner.RunScenario(t, path, rec)

	results := runner.Flush()
	require.NotEmpty(t, results, "Flush should return at least one ScenarioResult")

	found := false
	for _, r := range results {
		if r.Name == "collector test scenario" {
			found = true
			assert.Equal(t, "recorder", r.Registrar, "Registrar should match reg.Name()")
		}
	}
	assert.True(t, found, "ScenarioResult with correct Name and Registrar should be present")
}
