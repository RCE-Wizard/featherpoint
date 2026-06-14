package store

import (
	"context"
	"fmt"
)

// HostSoftwareRow is one row in the host→software report.
type HostSoftwareRow struct {
	CatalogID       string  `json:"catalog_id"`
	Name            string  `json:"name"`
	Publisher       *string `json:"publisher"`
	Version         *string `json:"version"`
	Source          string  `json:"source"`
	Signed          *bool   `json:"signed"`
	Signer          *string `json:"signer"`
	ExePath         *string `json:"exe_path"`
	InstallLocation *string `json:"install_location"`
	OwningUser      *string `json:"owning_user"`
	IsRunning       bool    `json:"is_running"`
	LastSeen        string  `json:"last_seen"`
}

// SoftwareHostRow is one row in the software→host report.
type SoftwareHostRow struct {
	HostID   string  `json:"host_id"`
	Hostname string  `json:"hostname"`
	OS       string  `json:"os"`
	Source   string  `json:"source"`
	LastSeen string  `json:"last_seen"`
}

// VersionSprawlRow is one version in the version-sprawl report.
type VersionSprawlRow struct {
	Version   *string `json:"version"`
	HostCount int     `json:"host_count"`
}

// UnsignedRow is one row in the unsigned-binaries report.
type UnsignedRow struct {
	HostID    string  `json:"host_id"`
	Hostname  string  `json:"hostname"`
	Name      string  `json:"name"`
	ExePath   *string `json:"exe_path"`
	Source    string  `json:"source"`
	LastSeen  string  `json:"last_seen"`
}

// DormantRow is a software item installed but never seen running.
type DormantRow struct {
	HostID          string  `json:"host_id"`
	Hostname        string  `json:"hostname"`
	Name            string  `json:"name"`
	Version         *string `json:"version"`
	Publisher       *string `json:"publisher"`
	InstallLocation *string `json:"install_location"`
	LastSeen        string  `json:"last_seen"`
}

// ReportFilter holds common pagination + filter params.
type ReportFilter struct {
	Limit  int
	Offset int
	Search string
}

func (f ReportFilter) limit() int {
	if f.Limit <= 0 || f.Limit > 500 {
		return 100
	}
	return f.Limit
}

// HostSoftware returns all software on a given host, optionally filtered by source.
func (db *DB) HostSoftware(ctx context.Context, hostID, source string, f ReportFilter) ([]HostSoftwareRow, int, error) {
	args := []interface{}{hostID}
	where := "hs.host_id = $1"
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND hs.source = $%d", len(args))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where += fmt.Sprintf(" AND sc.name ILIKE $%d", len(args))
	}

	countQ := fmt.Sprintf(`SELECT count(*) FROM host_software hs JOIN software_catalog sc ON sc.id=hs.catalog_id WHERE %s`, where)
	var total int
	if err := db.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.limit(), f.Offset)
	q := fmt.Sprintf(`
		SELECT sc.id, sc.name, sc.publisher, sc.version, hs.source, sc.signed, sc.signer,
		       hs.exe_path, hs.install_location, hs.owning_user, hs.is_running,
		       hs.last_seen::text
		FROM host_software hs JOIN software_catalog sc ON sc.id=hs.catalog_id
		WHERE %s
		ORDER BY sc.name, hs.source
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []HostSoftwareRow
	for rows.Next() {
		var r HostSoftwareRow
		if err := rows.Scan(&r.CatalogID, &r.Name, &r.Publisher, &r.Version,
			&r.Source, &r.Signed, &r.Signer, &r.ExePath, &r.InstallLocation,
			&r.OwningUser, &r.IsRunning, &r.LastSeen); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// SoftwareHosts returns hosts that have a given catalog entry.
func (db *DB) SoftwareHosts(ctx context.Context, catalogID string, f ReportFilter) ([]SoftwareHostRow, int, error) {
	var total int
	if err := db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM host_software hs JOIN hosts h ON h.id=hs.host_id WHERE hs.catalog_id=$1`,
		catalogID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT h.id, h.hostname, h.os, hs.source, hs.last_seen::text
		FROM host_software hs JOIN hosts h ON h.id=hs.host_id
		WHERE hs.catalog_id=$1
		ORDER BY h.hostname
		LIMIT $2 OFFSET $3`,
		catalogID, f.limit(), f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []SoftwareHostRow
	for rows.Next() {
		var r SoftwareHostRow
		if err := rows.Scan(&r.HostID, &r.Hostname, &r.OS, &r.Source, &r.LastSeen); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// VersionSprawl returns the version distribution for a named software.
func (db *DB) VersionSprawl(ctx context.Context, name string) ([]VersionSprawlRow, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT sc.version, count(DISTINCT hs.host_id)
		FROM host_software hs JOIN software_catalog sc ON sc.id=hs.catalog_id
		WHERE sc.name=$1
		GROUP BY sc.version
		ORDER BY count(DISTINCT hs.host_id) DESC`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VersionSprawlRow
	for rows.Next() {
		var r VersionSprawlRow
		if err := rows.Scan(&r.Version, &r.HostCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UnsignedBinaries returns running executables with signed=false.
func (db *DB) UnsignedBinaries(ctx context.Context, f ReportFilter) ([]UnsignedRow, int, error) {
	where := "hs.source='running' AND (sc.signed IS NULL OR sc.signed=false)"
	if f.Search != "" {
		where += fmt.Sprintf(" AND (sc.name ILIKE '%%%s%%' OR h.hostname ILIKE '%%%s%%')", f.Search, f.Search)
	}

	var total int
	if err := db.Pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT count(*) FROM host_software hs
			JOIN software_catalog sc ON sc.id=hs.catalog_id
			JOIN hosts h ON h.id=hs.host_id WHERE %s`, where),
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT h.id, h.hostname, sc.name, hs.exe_path, hs.source, hs.last_seen::text
		FROM host_software hs
		JOIN software_catalog sc ON sc.id=hs.catalog_id
		JOIN hosts h ON h.id=hs.host_id
		WHERE %s
		ORDER BY h.hostname, sc.name
		LIMIT $1 OFFSET $2`, where),
		f.limit(), f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []UnsignedRow
	for rows.Next() {
		var r UnsignedRow
		if err := rows.Scan(&r.HostID, &r.Hostname, &r.Name, &r.ExePath, &r.Source, &r.LastSeen); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// DormantSoftware returns software installed but never seen running.
func (db *DB) DormantSoftware(ctx context.Context, f ReportFilter) ([]DormantRow, int, error) {
	where := `hs.source='installed' AND NOT EXISTS (
		SELECT 1 FROM host_software hs2
		WHERE hs2.host_id=hs.host_id AND hs2.catalog_id=hs.catalog_id AND hs2.source='running'
	)`

	var total int
	if err := db.Pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT count(*) FROM host_software hs
			JOIN software_catalog sc ON sc.id=hs.catalog_id
			JOIN hosts h ON h.id=hs.host_id WHERE %s`, where),
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT h.id, h.hostname, sc.name, sc.version, sc.publisher, hs.install_location, hs.last_seen::text
		FROM host_software hs
		JOIN software_catalog sc ON sc.id=hs.catalog_id
		JOIN hosts h ON h.id=hs.host_id
		WHERE %s
		ORDER BY h.hostname, sc.name
		LIMIT $1 OFFSET $2`, where),
		f.limit(), f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []DormantRow
	for rows.Next() {
		var r DormantRow
		if err := rows.Scan(&r.HostID, &r.Hostname, &r.Name, &r.Version, &r.Publisher, &r.InstallLocation, &r.LastSeen); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// SoftwareCatalogSearch returns catalog entries matching a name search.
func (db *DB) SoftwareCatalogSearch(ctx context.Context, q string, f ReportFilter) ([]map[string]interface{}, int, error) {
	args := []interface{}{"%" + q + "%"}
	var total int
	if err := db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM software_catalog WHERE name ILIKE $1`, args[0],
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.limit(), f.Offset)
	rows, err := db.Pool.Query(ctx, `
		SELECT id, name, publisher, version, source, signed, signer, sha256
		FROM software_catalog WHERE name ILIKE $1
		ORDER BY name, version
		LIMIT $2 OFFSET $3`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var id, name, source string
		var publisher, version, signer, sha256 *string
		var signed *bool
		if err := rows.Scan(&id, &name, &publisher, &version, &source, &signed, &signer, &sha256); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"id": id, "name": name, "publisher": publisher, "version": version,
			"source": source, "signed": signed, "signer": signer, "sha256": sha256,
		})
	}
	return out, total, rows.Err()
}
