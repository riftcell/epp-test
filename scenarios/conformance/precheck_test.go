//go:build integration

// Package conformance_test provides an OT&E reachability pre-check that skips
// (not fails) integration tests when a registrar endpoint is unreachable.
// This satisfies CICD-03: the integration suite degrades gracefully in CI
// environments where OT&E sandbox access is not yet configured.
package conformance_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/riftcell/epp-test/pkg/config"
)

// skipIfUnreachable performs a TCP dial to host:port and calls t.Skipf if the
// endpoint is not reachable within 5 seconds.  It is a helper for
// TestOTEReachability subtests and may be reused by integration test suites.
func skipIfUnreachable(t *testing.T, host string, port int) {
	t.Helper()
	// net.JoinHostPort handles both IPv4 ("host:port") and IPv6 ("[host]:port").
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Skipf("registrar endpoint %s unreachable: %v", addr, err)
		return
	}
	_ = conn.Close()
}

// TestOTEReachability verifies that each configured registrar endpoint accepts
// a TCP connection.  A missing or invalid config file is treated as a skip, not
// a failure, so CI pipelines without OT&E credentials do not show red (CICD-03).
func TestOTEReachability(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("no registrar config available: %v", err)
		return
	}

	for name, rc := range cfg.Registrars {
		t.Run(name, func(t *testing.T) {
			skipIfUnreachable(t, rc.Host, rc.Port)
		})
	}
}
