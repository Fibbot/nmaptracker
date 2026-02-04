package db

import "fmt"

// InsertHostObservation stores one host observation row within a transaction.
func (tx *Tx) InsertHostObservation(obs HostObservation) (HostObservation, error) {
	var out HostObservation
	err := tx.QueryRow(
		`INSERT INTO host_observation (scan_import_id, project_id, ip_address, hostname, in_scope, host_state)
		 VALUES (?, ?, ?, ?, ?, ?)
		 RETURNING id, scan_import_id, project_id, ip_address, hostname, in_scope, host_state, created_at`,
		obs.ScanImportID, obs.ProjectID, obs.IPAddress, obs.Hostname, obs.InScope, obs.HostState,
	).Scan(&out.ID, &out.ScanImportID, &out.ProjectID, &out.IPAddress, &out.Hostname, &out.InScope, &out.HostState, &out.CreatedAt)
	if err != nil {
		return HostObservation{}, fmt.Errorf("insert host observation: %w", err)
	}
	return out, nil
}

// InsertPortObservation stores one port observation row within a transaction.
func (tx *Tx) InsertPortObservation(obs PortObservation) (PortObservation, error) {
	var out PortObservation
	err := tx.QueryRow(
		`INSERT INTO port_observation (
			scan_import_id, project_id, ip_address, port_number, protocol, state,
			service, version, product, extra_info, script_output
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING id, scan_import_id, project_id, ip_address, port_number, protocol, state,
		           service, version, product, extra_info, script_output, created_at`,
		obs.ScanImportID, obs.ProjectID, obs.IPAddress, obs.PortNumber, obs.Protocol, obs.State,
		obs.Service, obs.Version, obs.Product, obs.ExtraInfo, obs.ScriptOutput,
	).Scan(
		&out.ID, &out.ScanImportID, &out.ProjectID, &out.IPAddress, &out.PortNumber, &out.Protocol, &out.State,
		&out.Service, &out.Version, &out.Product, &out.ExtraInfo, &out.ScriptOutput, &out.CreatedAt,
	)
	if err != nil {
		return PortObservation{}, fmt.Errorf("insert port observation: %w", err)
	}
	return out, nil
}

// ListHostObservationsByImport returns host observations for one project/import pair.
func (db *DB) ListHostObservationsByImport(projectID, importID int64) ([]HostObservation, error) {
	rows, err := db.Query(
		`SELECT id, scan_import_id, project_id, ip_address, hostname, in_scope, host_state, created_at
		   FROM host_observation
		  WHERE project_id = ? AND scan_import_id = ?
		  ORDER BY ip_address`,
		projectID, importID,
	)
	if err != nil {
		return nil, fmt.Errorf("list host observations: %w", err)
	}
	defer rows.Close()

	var items []HostObservation
	for rows.Next() {
		var obs HostObservation
		if err := rows.Scan(&obs.ID, &obs.ScanImportID, &obs.ProjectID, &obs.IPAddress, &obs.Hostname, &obs.InScope, &obs.HostState, &obs.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan host observation: %w", err)
		}
		items = append(items, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list host observations rows: %w", err)
	}
	return items, nil
}

// ListPortObservationsByImport returns port observations for one project/import pair.
func (db *DB) ListPortObservationsByImport(projectID, importID int64) ([]PortObservation, error) {
	rows, err := db.Query(
		`SELECT id, scan_import_id, project_id, ip_address, port_number, protocol, state,
		        service, version, product, extra_info, script_output, created_at
		   FROM port_observation
		  WHERE project_id = ? AND scan_import_id = ?
		  ORDER BY ip_address, port_number, protocol`,
		projectID, importID,
	)
	if err != nil {
		return nil, fmt.Errorf("list port observations: %w", err)
	}
	defer rows.Close()

	var items []PortObservation
	for rows.Next() {
		var obs PortObservation
		if err := rows.Scan(
			&obs.ID, &obs.ScanImportID, &obs.ProjectID, &obs.IPAddress, &obs.PortNumber, &obs.Protocol, &obs.State,
			&obs.Service, &obs.Version, &obs.Product, &obs.ExtraInfo, &obs.ScriptOutput, &obs.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan port observation: %w", err)
		}
		items = append(items, obs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list port observations rows: %w", err)
	}
	return items, nil
}
