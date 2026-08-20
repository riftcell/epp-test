package rri

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

// generateSelfSignedCert generates an in-process self-signed TLS certificate
// for MockRRIServerTLS. No external tools or files are required (pure Go
// crypto/x509, CGO_ENABLED=0 compatible).
//
// Unlike pkg/mock/epp's generateTestCert, this builds neither a CA hierarchy
// nor a client certificate: DENIC RRI clients (including
// wcs-externalapi's rri.Client) dial with InsecureSkipVerify, so the server
// certificate is never validated by the connecting client. A bare
// self-signed leaf certificate is sufficient to satisfy the TLS handshake.
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("rri mock: generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "RRI Mock Server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}

	// Self-signed: the template signs itself (parent == template) instead of
	// being issued by a separate CA, since nothing on the client side ever
	// walks a chain back to a trust root.
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("rri mock: create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("rri mock: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("rri mock: build tls.Certificate: %w", err)
	}
	return cert, nil
}
