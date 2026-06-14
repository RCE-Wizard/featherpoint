package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// Service handles mTLS cert signing, enrollment token validation, and JWT issuance.
type Service struct {
	enrollmentToken string
	jwtSecret       []byte
	caKey           *rsa.PrivateKey
	caCert          *x509.Certificate
	caPEM           string
}

func New(enrollmentToken, caCertFile, caKeyFile string, jwtSecret []byte) (*Service, error) {
	s := &Service{enrollmentToken: enrollmentToken, jwtSecret: jwtSecret}

	if caCertFile != "" && caKeyFile != "" {
		if err := s.loadCA(caCertFile, caKeyFile); err != nil {
			return nil, err
		}
	} else {
		// Generate ephemeral CA for development
		if err := s.generateCA(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) generateCA() error {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}
	s.caKey = key

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Featherpoint CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	s.caCert, err = x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	s.caPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	return nil
}

func (s *Service) loadCA(certFile, keyFile string) error {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	s.caCert, err = x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	s.caKey = pair.PrivateKey.(*rsa.PrivateKey)
	s.caPEM = string(certPEM)
	return nil
}

// ValidEnrollmentToken checks a bearer token against the configured secret.
func (s *Service) ValidEnrollmentToken(token string) bool {
	return token == s.enrollmentToken
}

// SignCSR parses the CSR PEM, signs it with the CA, and returns (certPEM, fingerprint, error).
func (s *Service) SignCSR(csrPEM string) (string, string, error) {
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		return "", "", fmt.Errorf("invalid CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return "", "", fmt.Errorf("CSR signature: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(5 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, s.caCert, csr.PublicKey, s.caKey)
	if err != nil {
		return "", "", fmt.Errorf("sign cert: %w", err)
	}

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	fp := sha256.Sum256(der)
	fingerprint := hex.EncodeToString(fp[:])
	return certPEM, fingerprint, nil
}

// CAPEM returns the CA certificate in PEM format.
func (s *Service) CAPEM() string {
	return s.caPEM
}

// TLSConfig builds a server TLS config that requires mTLS from clients.
func (s *Service) TLSConfig(serverCertFile, serverKeyFile string) (*tls.Config, error) {
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(s.caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
