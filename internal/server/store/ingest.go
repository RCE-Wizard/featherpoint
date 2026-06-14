package store

import (
	"context"
	"fmt"
	"time"

	"github.com/featherpoint/swinv/internal/proto"
	"github.com/jackc/pgx/v5"
)

// IsBatchSeen returns true if (agentID, batchID) has already been processed.
func (db *DB) IsBatchSeen(ctx context.Context, agentID, batchID string) (bool, error) {
	var exists bool
	err := db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ingest_batches WHERE agent_id=$1 AND batch_id=$2)`,
		agentID, batchID,
	).Scan(&exists)
	return exists, err
}

// IngestBatch applies a set of software deltas for the given agent, within a transaction.
// It records the batch for idempotency and handles full-snapshot reconciliation.
func (db *DB) IngestBatch(ctx context.Context, agentID string, req *proto.IngestRequest) error {
	return pgx.BeginTxFunc(ctx, db.Pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Record batch for idempotency
		if _, err := tx.Exec(ctx,
			`INSERT INTO ingest_batches (agent_id, batch_id, received_at) VALUES ($1, $2, now())`,
			agentID, req.BatchID,
		); err != nil {
			return fmt.Errorf("record batch: %w", err)
		}

		// Fetch host_id from agent
		var hostID string
		if err := tx.QueryRow(ctx, `SELECT host_id FROM agents WHERE id=$1`, agentID).Scan(&hostID); err != nil {
			return fmt.Errorf("get host_id: %w", err)
		}

		now := time.Now().UTC()

		// Track catalog IDs seen in this batch per source (for full_snapshot reconcile)
		seenBySource := map[string][]string{} // source -> []catalog_id

		allDeltas := make([]proto.SoftwareDelta, 0, len(req.Running)+len(req.Installed))
		for i := range req.Running {
			req.Running[i].Source = "running"
			allDeltas = append(allDeltas, req.Running[i])
		}
		for i := range req.Installed {
			req.Installed[i].Source = "installed"
			allDeltas = append(allDeltas, req.Installed[i])
		}

		for _, d := range allDeltas {
			catalogID, err := upsertCatalog(ctx, tx, d, now)
			if err != nil {
				return fmt.Errorf("upsert catalog: %w", err)
			}

			if d.Op == "remove" {
				if _, err := tx.Exec(ctx,
					`DELETE FROM host_software WHERE host_id=$1 AND catalog_id=$2 AND source=$3`,
					hostID, catalogID, d.Source,
				); err != nil {
					return fmt.Errorf("remove host_software: %w", err)
				}
				continue
			}

			// upsert
			isRunning := d.Source == "running"
			if _, err := tx.Exec(ctx, `
				INSERT INTO host_software
					(host_id, catalog_id, source, exe_path, install_location, owning_user, is_running, first_seen, last_seen)
				VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now())
				ON CONFLICT (host_id, catalog_id, source)
				DO UPDATE SET
					exe_path=EXCLUDED.exe_path,
					install_location=EXCLUDED.install_location,
					owning_user=EXCLUDED.owning_user,
					is_running=EXCLUDED.is_running,
					last_seen=now()`,
				hostID, catalogID, d.Source,
				d.ExePath, d.InstallLocation, d.OwningUser, isRunning,
			); err != nil {
				return fmt.Errorf("upsert host_software: %w", err)
			}

			seenBySource[d.Source] = append(seenBySource[d.Source], catalogID)
		}

		// Full snapshot reconcile: remove rows not in this payload
		if req.FullSnapshot {
			for source, ids := range seenBySource {
				if _, err := tx.Exec(ctx, `
					DELETE FROM host_software
					WHERE host_id=$1 AND source=$2 AND catalog_id != ALL($3)`,
					hostID, source, ids,
				); err != nil {
					return fmt.Errorf("reconcile %s: %w", source, err)
				}
			}
		}

		return nil
	})
}

// upsertCatalog resolves or inserts a software_catalog row and returns its ID.
func upsertCatalog(ctx context.Context, tx pgx.Tx, d proto.SoftwareDelta, now time.Time) (string, error) {
	source := d.Source
	// If both running and installed paths see this, source becomes 'both' — simplified here;
	// a production version would merge. For MVP, store as-is.

	var id string
	if d.SHA256 != nil && *d.SHA256 != "" {
		// Running exe: dedupe by hash
		err := tx.QueryRow(ctx,
			`INSERT INTO software_catalog (source, name, publisher, version, sha256, signed, signer, arch, first_seen, last_seen)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now())
			 ON CONFLICT (sha256) WHERE sha256 IS NOT NULL
			 DO UPDATE SET last_seen=now(), source=EXCLUDED.source
			 RETURNING id`,
			source, d.Name, d.Publisher, d.Version, d.SHA256, d.Signed, d.Signer, d.Arch,
		).Scan(&id)
		if err != nil {
			return "", err
		}
	} else {
		// Installed package: dedupe by name+publisher+version
		pub := ""
		if d.Publisher != nil {
			pub = *d.Publisher
		}
		ver := ""
		if d.Version != nil {
			ver = *d.Version
		}
		err := tx.QueryRow(ctx,
			`INSERT INTO software_catalog (source, name, publisher, version, sha256, signed, signer, arch, first_seen, last_seen)
			 VALUES ($1,$2,$3,$4,NULL,$5,$6,$7,now(),now())
			 ON CONFLICT (name, coalesce(publisher,''), coalesce(version,'')) WHERE sha256 IS NULL
			 DO UPDATE SET last_seen=now(), source=EXCLUDED.source
			 RETURNING id`,
			source, d.Name, pub, ver, d.Signed, d.Signer, d.Arch,
		).Scan(&id)
		if err != nil {
			// Try select if insert failed (race) — rare but safe
			err2 := tx.QueryRow(ctx,
				`SELECT id FROM software_catalog WHERE sha256 IS NULL AND name=$1 AND coalesce(publisher,'')=$2 AND coalesce(version,'')=$3`,
				d.Name, pub, ver,
			).Scan(&id)
			if err2 != nil {
				return "", fmt.Errorf("catalog insert: %w (select fallback: %v)", err, err2)
			}
		}
	}
	_ = now
	return id, nil
}
