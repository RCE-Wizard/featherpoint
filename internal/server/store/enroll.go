package store

import (
	"context"
	"fmt"

	"github.com/featherpoint/swinv/internal/proto"
	"github.com/jackc/pgx/v5"
)

// EnrollResult is the data returned after a successful enrollment.
type EnrollResult struct {
	AgentID string
	HostID  string
}

// Enroll resolves or creates a host using the correlation waterfall, creates the agent row,
// and returns the new agent ID.
func (db *DB) Enroll(ctx context.Context, req *proto.EnrollRequest, certFingerprint string) (*EnrollResult, error) {
	var result EnrollResult
	err := pgx.BeginTxFunc(ctx, db.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		hostID, err := resolveOrCreateHost(ctx, tx, req.HostFacts)
		if err != nil {
			return fmt.Errorf("resolve host: %w", err)
		}
		result.HostID = hostID

		// Create agent row
		agentID := ""
		if err := tx.QueryRow(ctx, `
			INSERT INTO agents (host_id, agent_version, status, cert_fingerprint)
			VALUES ($1, $2, 'active', $3)
			RETURNING id`,
			hostID, req.AgentVersion, certFingerprint,
		).Scan(&agentID); err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		result.AgentID = agentID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// resolveOrCreateHost implements the correlation waterfall: serial -> fqdn -> mac -> hostname.
func resolveOrCreateHost(ctx context.Context, tx pgx.Tx, f proto.HostFacts) (string, error) {
	var hostID string

	// 1. Serial number (strongest signal)
	if f.SerialNumber != nil && *f.SerialNumber != "" {
		_ = tx.QueryRow(ctx, `SELECT id FROM hosts WHERE serial_number=$1`, f.SerialNumber).Scan(&hostID)
		if hostID != "" {
			return updateHost(ctx, tx, hostID, f)
		}
	}

	// 2. FQDN
	if f.FQDN != "" {
		_ = tx.QueryRow(ctx, `SELECT id FROM hosts WHERE lower(fqdn)=lower($1)`, f.FQDN).Scan(&hostID)
		if hostID != "" {
			return updateHost(ctx, tx, hostID, f)
		}
	}

	// 3. MAC addresses — any match
	if len(f.MACAddresses) > 0 {
		_ = tx.QueryRow(ctx, `SELECT id FROM hosts WHERE mac_addresses ?| $1 LIMIT 1`, f.MACAddresses).Scan(&hostID)
		if hostID != "" {
			return updateHost(ctx, tx, hostID, f)
		}
	}

	// 4. Hostname (weakest — could collide, but acceptable for MVP)
	_ = tx.QueryRow(ctx, `SELECT id FROM hosts WHERE hostname=$1 LIMIT 1`, f.Hostname).Scan(&hostID)
	if hostID != "" {
		return updateHost(ctx, tx, hostID, f)
	}

	// No match — create new host
	if err := tx.QueryRow(ctx, `
		INSERT INTO hosts (hostname, fqdn, os, os_version, serial_number, mac_addresses, primary_ip)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id`,
		f.Hostname, f.FQDN, f.OS, f.OSVersion, f.SerialNumber, macs(f.MACAddresses), f.PrimaryIP,
	).Scan(&hostID); err != nil {
		return "", fmt.Errorf("insert host: %w", err)
	}
	return hostID, nil
}

func updateHost(ctx context.Context, tx pgx.Tx, hostID string, f proto.HostFacts) (string, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE hosts SET
			hostname=$2, fqdn=$3, os=$4, os_version=$5,
			serial_number=COALESCE($6, serial_number),
			mac_addresses=$7, primary_ip=$8, last_seen=now()
		WHERE id=$1`,
		hostID, f.Hostname, f.FQDN, f.OS, f.OSVersion, f.SerialNumber, macs(f.MACAddresses), f.PrimaryIP,
	); err != nil {
		return "", fmt.Errorf("update host: %w", err)
	}
	return hostID, nil
}

// macs encodes MAC address slice as a JSON array string for pgx.
func macs(addrs []string) []byte {
	if len(addrs) == 0 {
		return []byte("[]")
	}
	b := []byte{'['}
	for i, a := range addrs {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '"')
		b = append(b, a...)
		b = append(b, '"')
	}
	b = append(b, ']')
	return b
}
