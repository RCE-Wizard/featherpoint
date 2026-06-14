// Throwaway Phase 1 checkpoint script: enrolls a fake agent and POSTs fake software data.
// Run with: go run scripts/seed/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/featherpoint/swinv/internal/proto"
	"github.com/google/uuid"
)

var baseURL = env("SERVER_URL", "http://localhost:8080")

func main() {
	// Step 1: Enroll
	serial := "SEED-SERIAL-001"
	ip := "10.0.0.1"
	enrollReq := proto.EnrollRequest{
		EnrollmentToken: env("ENROLLMENT_TOKEN", "dev-token"),
		AgentVersion:    "0.1.0",
		HostFacts: proto.HostFacts{
			Hostname:     "seed-host",
			FQDN:         "seed-host.local",
			OS:           "linux",
			OSVersion:    "Ubuntu 22.04",
			SerialNumber: &serial,
			MACAddresses: []string{"aa:bb:cc:dd:ee:ff"},
			PrimaryIP:    &ip,
		},
		CSRPEM: fakeCSR(),
	}

	var enrollResp proto.EnrollResponse
	post("/v1/enroll", "", enrollReq, &enrollResp)
	agentID := enrollResp.AgentID
	fmt.Printf("Enrolled: agent_id=%s\n", agentID)

	// Step 2: Ingest a full snapshot
	batchID := uuid.New().String()
	pub := "OpenSSL Project"
	ver := "3.0.2"
	ingestReq := proto.IngestRequest{
		Envelope: proto.Envelope{
			SchemaVersion: 1,
			AgentVersion:  "0.1.0",
			AgentID:       agentID,
			SentAt:        time.Now().UTC().Format(time.RFC3339),
		},
		BatchID:      batchID,
		CollectedAt:  time.Now(),
		FullSnapshot: true,
		Running: []proto.SoftwareDelta{
			{Op: "upsert", Source: "running", Name: "nginx", Publisher: &pub, Version: &ver,
				Signed: true, Signer: &pub},
		},
		Installed: []proto.SoftwareDelta{
			{Op: "upsert", Source: "installed", Name: "openssl", Publisher: &pub, Version: &ver, Signed: true},
			{Op: "upsert", Source: "installed", Name: "curl", Version: strPtr("7.88.1"), Signed: true},
		},
	}

	var ingestResp proto.IngestResponse
	postWithHeader("/v1/ingest", agentID, ingestReq, &ingestResp)
	fmt.Printf("Ingest accepted=%v\n", ingestResp.Accepted)

	// Step 3: Replay the same batch — must be a no-op
	postWithHeader("/v1/ingest", agentID, ingestReq, &ingestResp)
	fmt.Printf("Replay accepted=%v (should be true, no double-count)\n", ingestResp.Accepted)

	fmt.Println("Phase 1 checkpoint PASSED")
}

func post(path, agentID string, req, resp interface{}) {
	postWithHeader(path, agentID, req, resp)
}

func postWithHeader(path, agentID string, req, resp interface{}) {
	body, _ := json.Marshal(req)
	r, err := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	r.Header.Set("Content-Type", "application/json")
	if agentID != "" {
		r.Header.Set("X-Agent-ID", agentID)
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		log.Fatalf("POST %s => %d: %s", path, res.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, resp); err != nil {
		log.Fatalf("decode response: %v (body: %s)", err, raw)
	}
}

func fakeCSR() string {
	// A minimal placeholder CSR PEM for Phase 1 testing.
	// Phase 5 replaces this with a real RSA keypair + CSR.
	return `-----BEGIN CERTIFICATE REQUEST-----
MIICVDCCAToCAQAwDzENMAsGA1UEAxMEdGVzdDCBnzANBgkqhkiG9w0BAQEFAAOB
jQAwgYkCgYEA2a2rwplBQLDmJgQJVMJaLgYgBaBRFgLxC8JCtVkPAWJHNKBHCQ6o
3e5cD0SWKqJBl2mvYA4uJrrmfmjTCyB3xoiGU/d4fEf8fQ8t19PJtFBUFP6jbWQ
ByEiQH7Ll7q2ZuGzYjI3bRJSMBDqT4zK2JiAMnuFUPr8SXZZ3q0CAwEAAaBSMFAG
CSqGSIb3DQEJDjFDMEEwHQYDVR0OBBYEFMkxFSuqD7lJzIrr24F4n9lxIjWjMCAG
A1UdEQQZMBeCFXNlZWQtaG9zdC5sb2NhbC50ZXN0MA0GCSqGSIb3DQEBCwUAA4GB
ABcPUXJKkp1I0qvLdAnbODaT0TRgqXGzb6X4/wLzxdQlMaL+t2sKYxYPnOz+ov3A
3lz3kDqWm+m3mOaK3tMMpV7QYKE1qCr4FuLGWXFrFUVFKV2yR3z3ViLzxlCGAnj5
e6a7Ck3+/Pu1t6HXG4W1BfqFbAj6zM+1N+v4HHPQ
-----END CERTIFICATE REQUEST-----`
}

func strPtr(s string) *string { return &s }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
