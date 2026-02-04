package db

import (
	"database/sql"
	"fmt"
)

// Tx wraps sql.Tx to reuse DB helpers within a transaction.
type Tx struct {
	*sql.Tx
}

// Begin starts a transaction on the DB.
func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return &Tx{Tx: tx}, nil
}

// InsertScanImport records import metadata within a transaction.
func (tx *Tx) InsertScanImport(s ScanImport) (ScanImport, error) {
	var out ScanImport
	err := tx.QueryRow(
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

// InsertScanImportIntent records one intent tag for a scan import.
func (tx *Tx) InsertScanImportIntent(intent ScanImportIntent) (ScanImportIntent, error) {
	var out ScanImportIntent
	err := tx.QueryRow(
		`INSERT INTO scan_import_intent (scan_import_id, intent, source, confidence)
		 VALUES (?, ?, ?, ?)
		 RETURNING id, scan_import_id, intent, source, confidence, created_at`,
		intent.ScanImportID, intent.Intent, intent.Source, intent.Confidence,
	).Scan(&out.ID, &out.ScanImportID, &out.Intent, &out.Source, &out.Confidence, &out.CreatedAt)
	if err != nil {
		return ScanImportIntent{}, fmt.Errorf("insert scan import intent: %w", err)
	}
	return out, nil
}

// UpdateScanImportCounts updates the hosts/ports counts for an import within a transaction.
func (tx *Tx) UpdateScanImportCounts(id int64, hostsFound, portsFound int) error {
	_, err := tx.Exec(`UPDATE scan_import SET hosts_found = ?, ports_found = ? WHERE id = ?`, hostsFound, portsFound, id)
	if err != nil {
		return fmt.Errorf("update scan_import counts: %w", err)
	}
	return nil
}

// GetHostByIP fetches a host by project and IP within a transaction.
func (tx *Tx) GetHostByIP(projectID int64, ip string) (Host, bool, error) {
	var h Host
	err := tx.QueryRow(
		`SELECT id, project_id, ip_address, hostname, os_guess, latest_scan, in_scope, notes, created_at, updated_at
		 FROM host WHERE project_id = ? AND ip_address = ?`,
		projectID, ip,
	).Scan(&h.ID, &h.ProjectID, &h.IPAddress, &h.Hostname, &h.OSGuess, &h.LatestScan, &h.InScope, &h.Notes, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Host{}, false, nil
		}
		return Host{}, false, fmt.Errorf("get host: %w", err)
	}
	return h, true, nil
}

// UpsertHost inserts or updates a host keyed by (project_id, ip_address) within a transaction.
func (tx *Tx) UpsertHost(h Host) (Host, error) {
	var out Host
	var ipInt any
	if value, ok := ipv4ToInt(h.IPAddress); ok {
		ipInt = value
	}
	err := tx.QueryRow(
		`INSERT INTO host (project_id, ip_address, hostname, os_guess, in_scope, notes, ip_int)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, ip_address) DO UPDATE SET
		   hostname=excluded.hostname,
		   os_guess=excluded.os_guess,
		   in_scope=excluded.in_scope,
		   notes=excluded.notes,
		   ip_int=excluded.ip_int,
		   updated_at=CURRENT_TIMESTAMP
		 RETURNING id, project_id, ip_address, hostname, os_guess, latest_scan, in_scope, notes, created_at, updated_at`,
		h.ProjectID, h.IPAddress, h.Hostname, h.OSGuess, h.InScope, h.Notes, ipInt,
	).Scan(&out.ID, &out.ProjectID, &out.IPAddress, &out.Hostname, &out.OSGuess, &out.LatestScan, &out.InScope, &out.Notes, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Host{}, fmt.Errorf("upsert host: %w", err)
	}
	return out, nil
}

// GetPortByKey fetches a port by host/number/protocol within a transaction.
func (tx *Tx) GetPortByKey(hostID int64, portNumber int, protocol string) (Port, bool, error) {
	var p Port
	err := tx.QueryRow(
		`SELECT id, host_id, port_number, protocol, state, service, version, product, extra_info, work_status, script_output, notes, last_seen, created_at, updated_at
		 FROM port WHERE host_id = ? AND port_number = ? AND protocol = ?`,
		hostID, portNumber, protocol,
	).Scan(&p.ID, &p.HostID, &p.PortNumber, &p.Protocol, &p.State, &p.Service, &p.Version, &p.Product, &p.ExtraInfo, &p.WorkStatus, &p.ScriptOutput, &p.Notes, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Port{}, false, nil
		}
		return Port{}, false, fmt.Errorf("get port: %w", err)
	}
	return p, true, nil
}

// UpsertPort inserts or updates a port keyed by (host_id, port_number, protocol) within a transaction.
func (tx *Tx) UpsertPort(p Port) (Port, error) {
	var lastSeen any
	if !p.LastSeen.IsZero() {
		lastSeen = p.LastSeen
	}

	var out Port
	err := tx.QueryRow(
		`INSERT INTO port (host_id, port_number, protocol, state, service, version, product, extra_info, work_status, script_output, notes, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))
		 ON CONFLICT(host_id, port_number, protocol) DO UPDATE SET
		   state=excluded.state,
		   service=excluded.service,
		   version=excluded.version,
		   product=excluded.product,
		   extra_info=excluded.extra_info,
		   work_status=excluded.work_status,
		   script_output=excluded.script_output,
		   notes=excluded.notes,
		   last_seen=COALESCE(excluded.last_seen, port.last_seen, CURRENT_TIMESTAMP),
		   updated_at=CURRENT_TIMESTAMP
		 RETURNING id, host_id, port_number, protocol, state, service, version, product, extra_info, work_status, script_output, notes, last_seen, created_at, updated_at`,
		p.HostID, p.PortNumber, p.Protocol, p.State, p.Service, p.Version, p.Product, p.ExtraInfo, p.WorkStatus, p.ScriptOutput, p.Notes, lastSeen,
	).Scan(&out.ID, &out.HostID, &out.PortNumber, &out.Protocol, &out.State, &out.Service, &out.Version, &out.Product, &out.ExtraInfo, &out.WorkStatus, &out.ScriptOutput, &out.Notes, &out.LastSeen, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Port{}, fmt.Errorf("upsert port: %w", err)
	}
	return out, nil
}
