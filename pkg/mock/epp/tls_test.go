//go:build unit

package epp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTLSCert parses the leaf certificate from a tls.Certificate for inspection.
// Used in tests to assert on SAN fields.
func parseTLSCert(cert tls.Certificate) (*x509.Certificate, error) {
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("epp mock: tls.Certificate has no leaf DER bytes")
	}
	return x509.ParseCertificate(cert.Certificate[0])
}

// TestGenerateTestCertSANs verifies the generated cert covers both
// 127.0.0.1 (IP SAN) and localhost (DNS SAN) to prevent SNI mismatch.
func TestGenerateTestCertSANs(t *testing.T) {
	cert, pool, caKey, caCert, err := generateTestCert()
	require.NoError(t, err)
	require.NotNil(t, pool)
	require.NotNil(t, caKey)
	require.NotNil(t, caCert)

	// Parse the leaf certificate to inspect SANs
	require.NotEmpty(t, cert.Certificate)
	parsed, err := parseTLSCert(cert)
	require.NoError(t, err)

	// Must have 127.0.0.1 in IP SANs
	found127 := false
	for _, ip := range parsed.IPAddresses {
		if ip.Equal(net.ParseIP("127.0.0.1")) {
			found127 = true
		}
	}
	assert.True(t, found127, "cert must include IP SAN 127.0.0.1")

	// Must have localhost in DNS SANs
	assert.Contains(t, parsed.DNSNames, "localhost", "cert must include DNS SAN localhost")
}

// TestTLSDialWithGeneratedCert verifies a client can connect using InsecureSkipVerify=false
// with the CA pool returned from generateTestCert (no system cert store needed).
func TestTLSDialWithGeneratedCert(t *testing.T) {
	cert, pool, _, _, err := generateTestCert()
	require.NoError(t, err)

	serverCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	// Accept one connection in background; must complete the TLS handshake before
	// closing, otherwise the client receives EOF mid-handshake.
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		// Complete the TLS handshake so the client side does not get EOF.
		_ = c.(*tls.Conn).Handshake()
		c.Close()
	}()

	clientCfg := &tls.Config{
		RootCAs:    pool,
		ServerName: "127.0.0.1",
		// InsecureSkipVerify deliberately omitted — must verify via pool
	}
	conn, err := tls.Dial("tcp", ln.Addr().String(), clientCfg)
	require.NoError(t, err, "TLS dial with custom CA pool must succeed")
	conn.Close()
}

// TestGenerateClientCert verifies that GenerateClientCert produces a cert
// accepted by a server configured for mutual TLS.
func TestGenerateClientCert(t *testing.T) {
	// serverTLSCert, pool, caKey, and caCert all come from the same CA so the
	// client trusts the server cert (pool contains that CA) and the server trusts
	// the client cert (ClientCAs = same pool, client cert signed by same caKey/caCert).
	serverTLSCert, pool, caKey, caCert, err := generateTestCert()
	require.NoError(t, err)

	clientCert, err := GenerateClientCert(caKey, caCert)
	require.NoError(t, err)

	// Build a server that requires client cert
	serverCfg := &tls.Config{
		Certificates: []tls.Certificate{serverTLSCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	// Accept one connection; complete the TLS handshake (including client cert
	// verification) before closing so the dial side does not see EOF.
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		_ = c.(*tls.Conn).Handshake()
		c.Close()
	}()

	clientCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      pool,
		ServerName:   "127.0.0.1",
	}
	conn, err := tls.Dial("tcp", ln.Addr().String(), clientCfg)
	require.NoError(t, err, "mTLS dial with generated client cert must succeed")
	conn.Close()
}

// TestFaultTypes verifies MalformedFrameFault and WrongResultCodeFault
// can be placed in an any channel and type-switched correctly.
func TestFaultTypes(t *testing.T) {
	items := []any{
		MalformedFrameFault{},
		WrongResultCodeFault{Code: 2302},
		[]byte("normal response"),
	}

	for _, item := range items {
		switch v := item.(type) {
		case MalformedFrameFault:
			_ = v // expected
		case WrongResultCodeFault:
			assert.Equal(t, 2302, v.Code)
		case []byte:
			assert.Equal(t, []byte("normal response"), v)
		default:
			t.Fatalf("unexpected type %T", item)
		}
	}
}
