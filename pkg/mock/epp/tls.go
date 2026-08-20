package epp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// generateTestCert generates an in-process self-signed CA and server certificate
// for use by the EPP mock server. No external tools or files are required
// (MOCK-09: pure Go crypto/x509, CGO_ENABLED=0 compatible).
//
// The server cert includes SANs for 127.0.0.1 (IP) and localhost (DNS) so that
// TLS clients dialing either address can verify the cert without InsecureSkipVerify.
//
// Returns: server tls.Certificate, a CertPool containing the CA (for client RootCAs),
// the CA private key and CA certificate (needed by GenerateClientCert for mTLS).
func generateTestCert() (tls.Certificate, *x509.CertPool, *ecdsa.PrivateKey, *x509.Certificate, error) {
	// Generate CA key (ECDSA P-256: faster than RSA for test-time key generation)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: generate CA key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "EPP Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: create CA cert: %w", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: parse CA cert: %w", err)
	}

	// Generate server key + cert signed by CA.
	// Both 127.0.0.1 and localhost must be present — connecting via either address
	// without InsecureSkipVerify requires the matching SAN (Pitfall 4: SNI mismatch).
	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: generate server key: %w", err)
	}
	srvTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "EPP Mock Server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTemplate, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: create server cert: %w", err)
	}

	// PEM-encode server cert + key, then build tls.Certificate
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER})
	keyDER, err := x509.MarshalECPrivateKey(srvKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: marshal server key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, nil, nil, fmt.Errorf("epp mock: build tls.Certificate: %w", err)
	}

	// Build a CertPool containing only the test CA — clients set this as RootCAs
	// so InsecureSkipVerify can remain false while still trusting the test cert.
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return tlsCert, pool, caKey, caCert, nil
}

// GenerateClientCert generates a client certificate signed by the provided test CA.
// Phase 3 adapter tests call this to construct mTLS client connections against the
// EPP mock server (MOCK-01: mTLS support).
//
// Usage:
//
//	_, pool, caKey, caCert, _ := generateTestCert()
//	clientCert, _ := GenerateClientCert(caKey, caCert)
//	tlsCfg := &tls.Config{Certificates: []tls.Certificate{clientCert}, RootCAs: pool, ServerName: "127.0.0.1"}
func GenerateClientCert(caKey *ecdsa.PrivateKey, caCert *x509.Certificate) (tls.Certificate, error) {
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("epp mock: generate client key: %w", err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "EPP Mock Client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("epp mock: create client cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})
	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("epp mock: marshal client key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("epp mock: X509KeyPair: %w", err)
	}
	return cert, nil
}
