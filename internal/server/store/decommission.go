package store

import (
	"context"
	"encoding/json"
)

// DecommissionAgent marks an agent as decommissioned and queues the command.
func (db *DB) DecommissionAgent(ctx context.Context, agentID string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE agents SET status='decommissioned' WHERE id=$1`, agentID)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx,
		`INSERT INTO commands (agent_id, type, payload) VALUES ($1,'decommission','{}')`, agentID)
	return err
}

// PushConfig updates an agent's config and bumps config_version, queuing a config_update command.
func (db *DB) PushConfig(ctx context.Context, agentID string, cfg map[string]interface{}) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx,
		`UPDATE agents SET config=$2, config_version=config_version+1 WHERE id=$1`,
		agentID, data,
	)
	if err != nil {
		return err
	}
	_, err = db.Pool.Exec(ctx,
		`INSERT INTO commands (agent_id, type, payload) VALUES ($1,'config_update',$2)`,
		agentID, data,
	)
	return err
}
