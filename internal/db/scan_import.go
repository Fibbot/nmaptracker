package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// InsertScanImport records import metadata.
func (db *DB) InsertScanImport(s ScanImport) (ScanImport, error) {
	var out ScanImport
	err := db.QueryRow(
		`INSERT INTO scan_import (project_id, filename, hosts_found, ports_found)
		 VALUES (?, ?, ?, ?)
		 RETURNING id, project_id, filename, import_time, hosts_found, ports_found`,
		s.ProjectID, s.Filename, s.HostsFound, s.PortsFound,
	).Scan(&out.ID, &out.ProjectID, &out.Filename, &out.ImportTime, &out.HostsFound, &out.PortsFound)
	if err != nil {
		return ScanImport{}, fmt.Errorf("insert scan_import: %w", err)
	}
	return out, nil
}

// ListScanImports returns scan imports for a project ordered by id.
func (db *DB) ListScanImports(projectID int64) ([]ScanImport, error) {
	rows, err := db.Query(
		`SELECT id, project_id, filename, import_time, hosts_found, ports_found
		 FROM scan_import WHERE project_id = ? ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scan_import: %w", err)
	}
	defer rows.Close()

	var imports []ScanImport
	for rows.Next() {
		var s ScanImport
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Filename, &s.ImportTime, &s.HostsFound, &s.PortsFound); err != nil {
			return nil, fmt.Errorf("scan scan_import: %w", err)
		}
		imports = append(imports, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return imports, nil
}

// GetScanImportForProject fetches one scan import scoped to a project.
func (db *DB) GetScanImportForProject(projectID, importID int64) (ScanImport, bool, error) {
	var item ScanImport
	err := db.QueryRow(
		`SELECT id, project_id, filename, import_time, hosts_found, ports_found
		   FROM scan_import
		  WHERE id = ? AND project_id = ?`,
		importID, projectID,
	).Scan(&item.ID, &item.ProjectID, &item.Filename, &item.ImportTime, &item.HostsFound, &item.PortsFound)
	if err != nil {
		if err == sql.ErrNoRows {
			return ScanImport{}, false, nil
		}
		return ScanImport{}, false, fmt.Errorf("get scan import for project: %w", err)
	}
	return item, true, nil
}

// ListScanImportsWithIntents returns scan imports with their intent tags.
func (db *DB) ListScanImportsWithIntents(projectID int64) ([]ScanImportWithIntents, error) {
	rows, err := db.Query(
		`SELECT si.id, si.project_id, si.filename, si.import_time, si.hosts_found, si.ports_found,
		        sii.id, sii.scan_import_id, sii.intent, sii.source, sii.confidence, sii.created_at
		   FROM scan_import si
		   LEFT JOIN scan_import_intent sii ON sii.scan_import_id = si.id
		  WHERE si.project_id = ?
		  ORDER BY si.id, sii.id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scan imports with intents: %w", err)
	}
	defer rows.Close()

	var out []ScanImportWithIntents
	byID := make(map[int64]int)
	for rows.Next() {
		var item ScanImportWithIntents
		var intentID sql.NullInt64
		var intentScanImportID sql.NullInt64
		var intent sql.NullString
		var source sql.NullString
		var confidence sql.NullFloat64
		var createdAt sql.NullTime
		if err := rows.Scan(
			&item.ID,
			&item.ProjectID,
			&item.Filename,
			&item.ImportTime,
			&item.HostsFound,
			&item.PortsFound,
			&intentID,
			&intentScanImportID,
			&intent,
			&source,
			&confidence,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan import with intents: %w", err)
		}

		idx, ok := byID[item.ID]
		if !ok {
			idx = len(out)
			out = append(out, item)
			byID[item.ID] = idx
		}

		if intentID.Valid {
			out[idx].Intents = append(out[idx].Intents, ScanImportIntent{
				ID:           intentID.Int64,
				ScanImportID: intentScanImportID.Int64,
				Intent:       intent.String,
				Source:       source.String,
				Confidence:   confidence.Float64,
				CreatedAt:    createdAt.Time,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list scan imports with intents rows: %w", err)
	}
	return out, nil
}

// SetScanImportIntents replaces all intents for one import, scoped by project.
func (db *DB) SetScanImportIntents(projectID, importID int64, intents []ScanImportIntentInput) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM scan_import WHERE id = ? AND project_id = ?`, importID, projectID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return fmt.Errorf("check scan import ownership: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM scan_import_intent WHERE scan_import_id = ?`, importID); err != nil {
		return fmt.Errorf("delete scan import intents: %w", err)
	}

	seen := make(map[string]struct{})
	for _, input := range intents {
		intent := strings.TrimSpace(strings.ToLower(input.Intent))
		source := strings.TrimSpace(strings.ToLower(input.Source))
		confidence := input.Confidence
		if !ValidIntent(intent) {
			return fmt.Errorf("invalid intent %q", input.Intent)
		}
		if !ValidIntentSource(source) {
			return fmt.Errorf("invalid source %q", input.Source)
		}
		if confidence < 0 || confidence > 1 {
			return fmt.Errorf("invalid confidence %.3f", confidence)
		}
		if _, ok := seen[intent]; ok {
			return fmt.Errorf("duplicate intent %q", intent)
		}
		seen[intent] = struct{}{}

		if _, err := tx.Exec(
			`INSERT INTO scan_import_intent (scan_import_id, intent, source, confidence) VALUES (?, ?, ?, ?)`,
			importID, intent, source, confidence,
		); err != nil {
			return fmt.Errorf("insert scan import intent: %w", err)
		}
	}

	if err := syncHostLatestScanForImport(tx, projectID, importID); err != nil {
		return fmt.Errorf("sync host latest scan for import intents: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set scan import intents: %w", err)
	}
	return nil
}

func syncHostLatestScanForImport(tx *Tx, projectID, importID int64) error {
	rows, err := tx.Query(
		`SELECT DISTINCT ip_address
		   FROM host_observation
		  WHERE project_id = ? AND scan_import_id = ?`,
		projectID, importID,
	)
	if err != nil {
		return fmt.Errorf("list observed host ips for import: %w", err)
	}
	defer rows.Close()

	var ips []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return fmt.Errorf("scan observed host ip for import: %w", err)
		}
		ips = append(ips, ip)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate observed host ips for import: %w", err)
	}

	for _, ip := range ips {
		host, found, err := tx.GetHostByIP(projectID, ip)
		if err != nil {
			return fmt.Errorf("get host by ip for latest scan sync: %w", err)
		}
		if !found {
			continue
		}

		latestImportID, found, err := latestObservedImportForHost(tx, projectID, ip)
		if err != nil {
			return fmt.Errorf("latest observed import for host %s: %w", ip, err)
		}
		if !found {
			if _, err := tx.Exec(`UPDATE host SET latest_scan = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, HostLatestScanNone, host.ID); err != nil {
				return fmt.Errorf("clear host latest scan for host %s: %w", ip, err)
			}
			continue
		}

		latestScan, err := deriveLatestScanForImport(tx, latestImportID)
		if err != nil {
			return fmt.Errorf("derive latest scan for import %d: %w", latestImportID, err)
		}

		if _, err := tx.Exec(`UPDATE host SET latest_scan = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, latestScan, host.ID); err != nil {
			return fmt.Errorf("update host latest scan for host %s: %w", ip, err)
		}
	}

	return nil
}

func latestObservedImportForHost(tx *Tx, projectID int64, ip string) (int64, bool, error) {
	var importID int64
	err := tx.QueryRow(
		`SELECT ho.scan_import_id
		   FROM host_observation ho
		   JOIN scan_import si ON si.id = ho.scan_import_id
		  WHERE ho.project_id = ? AND ho.ip_address = ?
		  ORDER BY si.import_time DESC, ho.scan_import_id DESC
		  LIMIT 1`,
		projectID, ip,
	).Scan(&importID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("query latest observed import id: %w", err)
	}
	return importID, true, nil
}

func deriveLatestScanForImport(tx *Tx, importID int64) (string, error) {
	var hasAllTCP, hasTop1K, hasPingSweep int
	if err := tx.QueryRow(
		`SELECT
		     COALESCE(MAX(CASE WHEN intent = ? THEN 1 ELSE 0 END), 0),
		     COALESCE(MAX(CASE WHEN intent = ? THEN 1 ELSE 0 END), 0),
		     COALESCE(MAX(CASE WHEN intent = ? THEN 1 ELSE 0 END), 0)
		   FROM scan_import_intent
		  WHERE scan_import_id = ?`,
		IntentAllTCP,
		IntentTop1KTCP,
		IntentPingSweep,
		importID,
	).Scan(&hasAllTCP, &hasTop1K, &hasPingSweep); err != nil {
		return "", fmt.Errorf("query import intent flags: %w", err)
	}

	if hasAllTCP == 1 {
		return HostLatestScanFullPort, nil
	}
	if hasTop1K == 1 {
		return HostLatestScanTop1K, nil
	}
	if hasPingSweep == 1 {
		return HostLatestScanPingSweep, nil
	}
	return HostLatestScanNone, nil
}
