package sapmock

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"
)

// NewTLS starts a mock SAP API server over HTTPS that accepts X.509 client
// certificates issued by its own certificate authority, as well as basic
// authentication. A certificate authenticates the communication user named
// by its common name. Close the server when done.
func NewTLS() *Server {
	ca, caKey := newCertificateAuthority()
	s := newServer()
	s.ca, s.caKey = ca, caKey

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca)
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(s.handle))
	s.TLS = &tls.Config{ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: clientCAs, MinVersion: tls.VersionTLS12}
	s.StartTLS()
	return s
}

// RootCAs returns a pool that trusts the server's certificate.
func (s *Server) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return pool
}

// ClientCertificate issues a PEM encoded client certificate and key for the
// communication user cn, trusted by a server started with NewTLS.
func (s *Server) ClientCertificate(cn string) (certPEM, keyPEM string) {
	return issueCertificate(cn, s.ca, s.caKey)
}

// UntrustedClientCertificate returns a self-signed client certificate and key
// that no mock server trusts.
func UntrustedClientCertificate(cn string) (certPEM, keyPEM string) {
	return issueCertificate(cn, nil, nil)
}

// clientCertificateUser returns the communication user authenticated by a
// verified client certificate, if one was presented.
func clientCertificateUser(r *http.Request) (string, bool) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		return "", false
	}
	return r.TLS.VerifiedChains[0][0].Subject.CommonName, true
}

func newCertificateAuthority() (*x509.Certificate, *ecdsa.PrivateKey) {
	key := mustKey()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sapmock CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	return ca, key
}

// issueCertificate creates a client certificate signed by ca, or a
// self-signed one when ca is nil.
func issueCertificate(cn string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) (certPEM, keyPEM string) {
	key := mustKey()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	parent, signer := template, key
	if ca != nil {
		parent, signer = ca, caKey
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		panic(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

func mustKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return key
}
