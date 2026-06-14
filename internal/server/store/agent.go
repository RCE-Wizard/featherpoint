package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/featherpoint/swinv/internal/proto"
)

// AgentRow is a full agent record from the database.
type AgentRow struct {
	ID              string
	HostID          string
	AgentVersion    string
	Status          string
	CertFingerprint *string
	Config          proto.AgentConfig
	ConfigVersion   int
	EnrolledAt      time.Time
	LastCheckin     *time.Time
	LastHeartbeat   *time.Time
}

// GetAgentByCert returns an agent by its client cert fingerprint.
func (db *DB) GetAgentByCert(ctx context.Context, fingerprint string) (*AgentRow, error) {
	row := &AgentRow{}
	var rawConfig []byte
	err := db.Pool.QueryRow(ctx, `
		SELECT id, host_id, agent_version, status, cert_fingerprint,
		       config, config_version, enrolled_at, last_checkin, last_heartbeat
		FROM agents WHERE cert_fingerprint=$1 AND status='active'`,
		fingerprint,
	).Scan(&row.ID, &row.HostID, &row.AgentVersion, &row.Status, &row.CertFingerprint,
		&rawConfig, &row.ConfigVersion, &row.EnrolledAt, &row.LastCheckin, &row.LastHeartbeat)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(rawConfig, &row.Config)
	return row, nil
}

// GetAgentByID returns an agent by its UUID.
func (db *DB) GetAgentByID(ctx context.Context, agentID string) (*AgentRow, error) {
	row := &AgentRow{}
	var rawConfig []byte
	err := db.Pool.QueryRow(ctx, `
		SELECT id, host_id, agent_version, status, cert_fingerprint,
		       config, config_version, enrolled_at, last_checkin, last_heartbeat
		FROM agents WHERE id=$1`,
		agentID,
	).Scan(&row.ID, &row.HostID, &row.AgentVersion, &row.Status, &row.CertFingerprint,
		&rawConfig, &row.ConfigVersion, &row.EnrolledAt, &row.LastCheckin, &row.LastHeartbeat)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(rawConfig, &row.Config)
	return row, nil
}

// TouchCheckin updates last_checkin for an agent.
func (db *DB) TouchCheckin(ctx context.Context, agentID string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE agents SET last_checkin=now() WHERE id=$1`, agentID)
	return err
}

// TouchHeartbeat updates last_heartbeat and last_metrics.
func (db *DB) TouchHeartbeat(ctx context.Context, agentID string, metrics proto.AgentMetrics) error {
	data, _ := json.Marshal(metrics)
	_, err := db.Pool.Exec(ctx,
		`UPDATE agents SET last_heartbeat=now(), last_metrics=$2 WHERE id=$1`,
		agentID, data,
	)
	return err
}

// PendingCommands returns unacked commands for an agent.
func (db *DB) PendingCommands(ctx context.Context, agentID string) ([]proto.Command, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, type, payload FROM commands WHERE agent_id=$1 AND status='pending' ORDER BY created_at`,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []proto.Command
	for rows.Next() {
		var c proto.Command
		var rawPayload []byte
		if err := rows.Scan(&c.ID, &c.Type, &rawPayload); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawPayload, &c.Payload)
		cmds = append(cmds, c)
	}
	return cmds, rows.Err()
}

// AckCommands marks commands as acked.
func (db *DB) AckCommands(ctx context.Context, agentID string, commandIDs []string) error {
	if len(commandIDs) == 0 {
		return nil
	}
	_, err := db.Pool.Exec(ctx,
		`UPDATE commands SET status='acked', acked_at=now() WHERE agent_id=$1 AND id=ANY($2)`,
		agentID, commandIDs,
	)
	return err
}

// ListAgents returns all agents with their host info for the fleet view.
func (db *DB) ListAgents(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT a.id, a.agent_version, a.status, a.config_version,
		       a.enrolled_at, a.last_checkin, a.last_heartbeat,
		       h.hostname, h.os, h.primary_ip
		FROM agents a JOIN hosts h ON h.id=a.host_id
		ORDER BY h.hostname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var (
			id, ver, status string
			cfgVer          int
			enrolled        time.Time
			checkin, hb     *time.Time
			hostname, os_, ip *string
		)
		if err := rows.Scan(&id, &ver, &status, &cfgVer, &enrolled, &checkin, &hb, &hostname, &os_, &ip); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "agent_version": ver, "status": status,
			"config_version": cfgVer, "enrolled_at": enrolled,
			"last_checkin": checkin, "last_heartbeat": hb,
			"hostname": hostname, "os": os_, "primary_ip": ip,
		})
	}
	return out, rows.Err()
}
