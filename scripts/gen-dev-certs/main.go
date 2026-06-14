// Generates a self-signed CA + server cert for local development.
// Run: go run scripts/gen-dev-certs/main.go
// Outputs: deploy/dev-ca.crt, deploy/dev-ca.key, deploy/dev-server.crt, deploy/dev-server.key
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	os.MkdirAll("deploy/certs", 0700)

	// CA
	caKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Featherpoint Dev CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)

	writePEM("deploy/certs/ca.crt", "CERTIFICATE", caDER)
	writePEM("deploy/certs/ca.key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey))
	log.Println("wrote deploy/certs/ca.crt + ca.key")

	// Server cert
	srvKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(2 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	srvDER, _ := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)

	writePEM("deploy/certs/server.crt", "CERTIFICATE", srvDER)
	writePEM("deploy/certs/server.key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey))
	log.Println("wrote deploy/certs/server.crt + server.key")

	log.Println("\nStart server with:")
	log.Println("  CA_CERT=deploy/certs/ca.crt CA_KEY=deploy/certs/ca.key \\")
	log.Println("  TLS_CERT=deploy/certs/server.crt TLS_KEY=deploy/certs/server.key \\")
	log.Println("  ./bin/swinv-server")
	log.Println("\nAgent config.json:")
	log.Println(`  { "server_url": "https://localhost:8080", "enrollment_token": "dev-token", "insecure_skip_verify": true }`)
}

func writePEM(path, typ string, der []byte) {
	f, _ := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: typ, Bytes: der})
}
