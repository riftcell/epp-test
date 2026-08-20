//go:build unit

package fault_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/fault"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestParseDurations(t *testing.T) {
	tests := []struct {
		name    string
		profile fault.FaultProfile
		wantErr bool
		check   func(t *testing.T, p fault.FaultProfile)
	}{
		{
			name:    "valid connect_delay 2s",
			profile: fault.FaultProfile{ConnectDelay: "2s"},
			check: func(t *testing.T, p fault.FaultProfile) {
				t.Helper()
				// Verify round-trip: parse then call OnConnect on a zero-delay engine should not sleep.
				// We confirm no error was returned (the duration parsed correctly).
			},
		},
		{
			name:    "valid response_delay 100ms",
			profile: fault.FaultProfile{ResponseDelay: "100ms"},
		},
		{
			name: "valid per_op delay 400ms",
			profile: fault.FaultProfile{
				PerOp: []fault.PerOpRule{{Match: "domain:check", Delay: "400ms"}},
			},
			check: func(t *testing.T, p fault.FaultProfile) {
				t.Helper()
				// Verify the per-op delay is applied by checking OnOperation returns 400ms.
				engine := fault.NewFaultEngine(p)
				_, delay := engine.OnOperation("domain:check")
				assert.Equal(t, 400*time.Millisecond, delay, "per-op delay must resolve to 400ms")
			},
		},
		{
			name:    "valid empty strings are no-op",
			profile: fault.FaultProfile{ConnectDelay: "", ResponseDelay: ""},
		},
		{
			name:    "invalid connect_delay bad string",
			profile: fault.FaultProfile{ConnectDelay: "bad"},
			wantErr: true,
		},
		{
			name: "invalid per_op delay xyz",
			profile: fault.FaultProfile{
				PerOp: []fault.PerOpRule{{Match: "x", Delay: "xyz"}},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.profile
			err := p.ParseDurations()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.check != nil {
				tc.check(t, p)
			}
		})
	}
}

func TestOnLoginAttemptModes(t *testing.T) {
	tests := []struct {
		name        string
		profile     fault.FaultProfile
		wantActions []fault.FaultAction
	}{
		{
			name:        "always - single call returns deny",
			profile:     fault.FaultProfile{LoginMode: "always"},
			wantActions: []fault.FaultAction{fault.ActionDeny},
		},
		{
			name:        "always - three calls all deny",
			profile:     fault.FaultProfile{LoginMode: "always"},
			wantActions: []fault.FaultAction{fault.ActionDeny, fault.ActionDeny, fault.ActionDeny},
		},
		{
			name:           "flap - first two fail then allow",
			profile:        fault.FaultProfile{LoginMode: "flap", LoginFailCount: 2},
			wantActions:    []fault.FaultAction{fault.ActionDeny, fault.ActionDeny, fault.ActionAllow},
		},
		{
			name:        "hang mode returns hang",
			profile:     fault.FaultProfile{LoginMode: "hang"},
			wantActions: []fault.FaultAction{fault.ActionHang},
		},
		{
			name:        "disconnect mode returns disconnect",
			profile:     fault.FaultProfile{LoginMode: "disconnect"},
			wantActions: []fault.FaultAction{fault.ActionDisconnect},
		},
		{
			name:        "empty mode returns allow",
			profile:     fault.FaultProfile{LoginMode: ""},
			wantActions: []fault.FaultAction{fault.ActionAllow},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := fault.NewFaultEngine(tc.profile)
			for i, want := range tc.wantActions {
				got, err := engine.OnLoginAttempt(nil)
				require.NoError(t, err)
				assert.Equal(t, want, got, "call %d", i+1)
			}
		})
	}
}

func TestOnConnectDelay(t *testing.T) {
	profile := fault.FaultProfile{ConnectDelay: "50ms"}
	require.NoError(t, profile.ParseDurations())
	engine := fault.NewFaultEngine(profile)

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() }) //nolint:errcheck // best-effort close in Cleanup; see CONVENTIONS.md §3

	start := time.Now()
	action, err := engine.OnConnect(server)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, fault.ActionAllow, action)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestOnConnectPreGreeting(t *testing.T) {
	profile := fault.FaultProfile{DisconnectAt: "pre-greeting"}
	engine := fault.NewFaultEngine(profile)

	action, err := engine.OnConnect(nil)
	require.NoError(t, err)
	assert.Equal(t, fault.ActionDisconnect, action)
}

func TestAfterGreeting(t *testing.T) {
	tests := []struct {
		name        string
		disconnectAt string
		want        fault.FaultAction
	}{
		{
			name:         "post-greeting returns disconnect",
			disconnectAt: "post-greeting",
			want:         fault.ActionDisconnect,
		},
		{
			name:         "pre-greeting returns allow (pre-greeting handled by OnConnect)",
			disconnectAt: "pre-greeting",
			want:         fault.ActionAllow,
		},
		{
			name:         "empty returns allow",
			disconnectAt: "",
			want:         fault.ActionAllow,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := fault.FaultProfile{DisconnectAt: tc.disconnectAt}
			engine := fault.NewFaultEngine(profile)
			assert.Equal(t, tc.want, engine.AfterGreeting())
		})
	}
}

func TestOnOperationDisconnectAfterN(t *testing.T) {
	profile := fault.FaultProfile{DisconnectAfter: 3}
	engine := fault.NewFaultEngine(profile)

	// Ops 1, 2, 3 must allow.
	for i := 0; i < 3; i++ {
		action, _ := engine.OnOperation("<domain:check>")
		assert.Equal(t, fault.ActionAllow, action, "op %d should allow", i+1)
	}

	// Op 4 triggers disconnect; delay must be zero.
	action, delay := engine.OnOperation("<domain:check>")
	assert.Equal(t, fault.ActionDisconnect, action)
	assert.Zero(t, delay)
}

func TestOnOperationPerOpMatch(t *testing.T) {
	profile := fault.FaultProfile{
		ResponseDelay: "100ms",
		PerOp:         []fault.PerOpRule{{Match: "domain:create", Delay: "400ms", Mismatch: true}},
	}
	require.NoError(t, profile.ParseDurations())
	engine := fault.NewFaultEngine(profile)

	// Non-matching op: only global delay, no mismatch.
	action, delay := engine.OnOperation("<domain:check>whatever</domain:check>")
	assert.Equal(t, fault.ActionAllow, action)
	assert.Equal(t, 100*time.Millisecond, delay, "only global delay for non-matching op")

	// Matching op: global + per-op stacked, mismatch.
	action, delay = engine.OnOperation("<domain:create>example.com</domain:create>")
	assert.Equal(t, fault.ActionMismatch, action)
	assert.Equal(t, 500*time.Millisecond, delay, "global 100ms + per-op 400ms = 500ms")
}

func TestOnOperationGlobalMismatch(t *testing.T) {
	profile := fault.FaultProfile{FaultMismatch: "domain:create"}
	engine := fault.NewFaultEngine(profile)

	action, _ := engine.OnOperation("<domain:check>example.com</domain:check>")
	assert.Equal(t, fault.ActionAllow, action, "no match: allow")

	action, _ = engine.OnOperation("<domain:create>example.com</domain:create>")
	assert.Equal(t, fault.ActionMismatch, action, "global mismatch match")
}

func TestOnOperationPerOpDisconnect(t *testing.T) {
	profile := fault.FaultProfile{
		PerOp: []fault.PerOpRule{{Match: "domain:delete", Disconnect: true}},
	}
	engine := fault.NewFaultEngine(profile)

	action, delay := engine.OnOperation("<domain:delete>example.com</domain:delete>")
	assert.Equal(t, fault.ActionDisconnect, action)
	assert.Zero(t, delay)
}

func TestApplyResponseFaultLengthOverflow(t *testing.T) {
	profile := fault.FaultProfile{MalformedFrame: "length_overflow"}
	engine := fault.NewFaultEngine(profile)

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() }) //nolint:errcheck // best-effort close in Cleanup; see CONVENTIONS.md §3

	body := []byte("test-body")
	var readBuf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&readBuf, client) //nolint:errcheck // test helper: copy until EOF
	}()

	result, action := engine.ApplyResponseFault(server, body)
	assert.Nil(t, result)
	assert.Equal(t, fault.ActionDisconnect, action)
	_ = server.Close() //nolint:errcheck // best-effort close; signals EOF to the reader goroutine
	<-done

	written := readBuf.Bytes()
	require.GreaterOrEqual(t, len(written), 4)
	claimedTotal := binary.BigEndian.Uint32(written[:4])
	assert.Equal(t, uint32(len(body)+4+100), claimedTotal, "header must overclaim by 100")
	assert.Equal(t, body, written[4:], "body bytes after header must be unmodified")
}

func TestApplyResponseFaultInvalidXML(t *testing.T) {
	profile := fault.FaultProfile{MalformedFrame: "invalid_xml"}
	engine := fault.NewFaultEngine(profile)

	result, action := engine.ApplyResponseFault(nil, []byte("normal"))
	assert.Equal(t, fault.ActionAllow, action)
	assert.Contains(t, string(result), "not valid xml")
}

func TestApplyResponseFaultGarbage(t *testing.T) {
	profile := fault.FaultProfile{MalformedFrame: "garbage"}
	engine := fault.NewFaultEngine(profile)

	result, action := engine.ApplyResponseFault(nil, []byte("normal"))
	assert.Equal(t, fault.ActionAllow, action)
	assert.Equal(t, []byte{0xFF, 0xFE, 0x00, 0x01, 0xDE, 0xAD, 0xBE, 0xEF}, result)
}

func TestApplyResponseFaultFiredGuard(t *testing.T) {
	profile := fault.FaultProfile{MalformedFrame: "garbage"}
	engine := fault.NewFaultEngine(profile)

	normal := []byte("<epp>normal</epp>")
	first, _ := engine.ApplyResponseFault(nil, normal)
	assert.NotEqual(t, normal, first, "first call must return garbage bytes")

	second, action2 := engine.ApplyResponseFault(nil, normal)
	assert.Equal(t, fault.ActionAllow, action2)
	assert.Equal(t, normal, second, "second call must return original body unchanged")
}
