package store

import (
	"context"
	"encoding/json"
)

// CreateCommand inserts a new management command and returns its UUID.
func (db *DB) CreateCommand(ctx context.Context, agentID, cmdType string, payload map[string]interface{}) (string, error) {
	data, _ := json.Marshal(payload)
	var id string
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO commands (agent_id, type, payload) VALUES ($1,$2,$3) RETURNING id`,
		agentID, cmdType, data,
	).Scan(&id)
	return id, err
}

// WriteAuditLog records a web-app action in audit_log.
func (db *DB) WriteAuditLog(ctx context.Context, actor, action, target string, detail map[string]interface{}) error {
	data, _ := json.Marshal(detail)
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO audit_log (actor, action, target, detail) VALUES ($1,$2,$3,$4)`,
		actor, action, target, data,
	)
	return err
}
