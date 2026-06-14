// Package enroll handles first-run agent enrollment: keypair generation, CSR, and cert storage.
package enroll

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const keyBits = 4096

// Paths returns the on-disk paths for the agent's TLS material.
func Paths(dataDir string) (keyPath, certPath, caPath string) {
	return filepath.Join(dataDir, "agent.key"),
		filepath.Join(dataDir, "agent.crt"),
		filepath.Join(dataDir, "ca.crt")
}

// IsEnrolled returns true if the agent already has a client cert on disk.
func IsEnrolled(dataDir string) bool {
	_, certPath, _ := Paths(dataDir)
	_, err := os.Stat(certPath)
	return err == nil
}

// GenerateKeyAndCSR generates a new RSA keypair, writes the key to disk (0600),
// and returns the CSR PEM string to send to /enroll.
func GenerateKeyAndCSR(dataDir, hostname string) (csrPEM string, err error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", fmt.Errorf("mkdir dataDir: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	keyPath, _, _ := Paths(dataDir)
	if err := writePrivate(keyPath, x509.MarshalPKCS1PrivateKey(key)); err != nil {
		return "", fmt.Errorf("write key: %w", err)
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: hostname},
	}, key)
	if err != nil {
		return "", fmt.Errorf("create CSR: %w", err)
	}
	csrPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr}))
	return csrPEM, nil
}

// StoreCerts writes the server-issued client cert and CA cert to disk (0600 each).
func StoreCerts(dataDir, clientCertPEM, caPEM string) error {
	_, certPath, caPath := Paths(dataDir)
	if err := writeFile(certPath, []byte(clientCertPEM)); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := writeFile(caPath, []byte(caPEM)); err != nil {
		return fmt.Errorf("write CA: %w", err)
	}
	return nil
}

// TLSConfig builds a client TLS config from the stored key, cert, and CA.
func TLSConfig(dataDir string) (*tls.Config, error) {
	keyPath, certPath, caPath := Paths(dataDir)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("parse CA cert")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

func writePrivate(path string, der []byte) error {
	return writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
