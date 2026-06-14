package store

import (
	"context"
	"time"
)

type AuditRow struct {
	ID     int64                  `json:"id"`
	Actor  string                 `json:"actor"`
	Action string                 `json:"action"`
	Target string                 `json:"target"`
	Detail map[string]interface{} `json:"detail"`
	At     time.Time              `json:"at"`
}

func (db *DB) AuditLog(ctx context.Context, limit, offset int) ([]AuditRow, int, error) {
	var total int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := db.Pool.Query(ctx,
		`SELECT id, actor, action, target, detail, at FROM audit_log ORDER BY at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		var rawDetail []byte
		if err := rows.Scan(&r.ID, &r.Actor, &r.Action, &r.Target, &rawDetail, &r.At); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}
