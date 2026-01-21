package db

import "fmt"

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
