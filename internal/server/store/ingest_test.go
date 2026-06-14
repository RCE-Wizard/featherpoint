package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/featherpoint/swinv/internal/proto"
	"github.com/featherpoint/swinv/internal/server/store"
	"github.com/google/uuid"
)

func testDB(t *testing.T) *store.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestIngestIdempotency(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Set up a host and agent directly
	agentID, _ := seedAgent(t, db, ctx)

	pub := "Test Pub"
	ver := "1.0"
	req := &proto.IngestRequest{
		Envelope: proto.Envelope{
			SchemaVersion: 1,
			AgentVersion:  "0.1.0",
			AgentID:       agentID,
			SentAt:        time.Now().UTC().Format(time.RFC3339),
		},
		BatchID:      uuid.New().String(),
		CollectedAt:  time.Now(),
		FullSnapshot: false,
		Installed: []proto.SoftwareDelta{
			{Op: "upsert", Source: "installed", Name: "curl", Publisher: &pub, Version: &ver, Signed: true},
		},
	}

	// First ingest
	if err := db.IngestBatch(ctx, agentID, req); err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	// Second ingest with same batch_id — must be idempotent (IsBatchSeen prevents re-apply)
	seen, err := db.IsBatchSeen(ctx, agentID, req.BatchID)
	if err != nil {
		t.Fatalf("IsBatchSeen: %v", err)
	}
	if !seen {
		t.Fatal("expected batch to be seen after first ingest")
	}
}

func TestFullSnapshotReconcile(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	agentID, _ := seedAgent(t, db, ctx)

	pub := "Pkg"
	v1, v2 := "1.0", "2.0"

	// Ingest two packages
	req1 := &proto.IngestRequest{
		Envelope:     proto.Envelope{SchemaVersion: 1, AgentVersion: "0.1.0", AgentID: agentID, SentAt: time.Now().UTC().Format(time.RFC3339)},
		BatchID:      uuid.New().String(),
		CollectedAt:  time.Now(),
		FullSnapshot: true,
		Installed: []proto.SoftwareDelta{
			{Op: "upsert", Source: "installed", Name: "pkg-a", Publisher: &pub, Version: &v1},
			{Op: "upsert", Source: "installed", Name: "pkg-b", Publisher: &pub, Version: &v2},
		},
	}
	if err := db.IngestBatch(ctx, agentID, req1); err != nil {
		t.Fatalf("ingest req1: %v", err)
	}

	// Full snapshot with only pkg-a — pkg-b should be removed
	req2 := &proto.IngestRequest{
		Envelope:     proto.Envelope{SchemaVersion: 1, AgentVersion: "0.1.0", AgentID: agentID, SentAt: time.Now().UTC().Format(time.RFC3339)},
		BatchID:      uuid.New().String(),
		CollectedAt:  time.Now(),
		FullSnapshot: true,
		Installed: []proto.SoftwareDelta{
			{Op: "upsert", Source: "installed", Name: "pkg-a", Publisher: &pub, Version: &v1},
		},
	}
	if err := db.IngestBatch(ctx, agentID, req2); err != nil {
		t.Fatalf("ingest req2: %v", err)
	}

	// Verify pkg-b is gone from host_software
	var count int
	err := db.Pool.QueryRow(ctx, `
		SELECT count(*) FROM host_software hs
		JOIN software_catalog sc ON sc.id=hs.catalog_id
		JOIN agents a ON a.id=$1
		WHERE hs.host_id=a.host_id AND sc.name='pkg-b' AND hs.source='installed'`,
		agentID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count pkg-b: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected pkg-b removed, got count=%d", count)
	}
}

// seedAgent creates a host+agent row and returns (agentID, hostID).
func seedAgent(t *testing.T, db *store.DB, ctx context.Context) (string, string) {
	t.Helper()
	serial := "test-" + uuid.New().String()
	enrollReq := &proto.EnrollRequest{
		EnrollmentToken: "test",
		AgentVersion:    "0.1.0",
		HostFacts: proto.HostFacts{
			Hostname:     "test-host-" + uuid.New().String()[:8],
			FQDN:         "test.local",
			OS:           "linux",
			OSVersion:    "Ubuntu 22.04",
			SerialNumber: &serial,
			MACAddresses: []string{"aa:bb:cc:dd:ee:" + uuid.New().String()[:2]},
		},
		CSRPEM: "placeholder",
	}
	result, err := db.Enroll(ctx, enrollReq, uuid.New().String())
	if err != nil {
		t.Fatalf("seedAgent enroll: %v", err)
	}
	return result.AgentID, result.HostID
}
