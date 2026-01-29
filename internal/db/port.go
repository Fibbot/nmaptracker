package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// UpsertPort inserts or updates a port keyed by (host_id, port_number, protocol).
func (db *DB) UpsertPort(p Port) (Port, error) {
	var lastSeen any
	if !p.LastSeen.IsZero() {
		lastSeen = p.LastSeen
	}

	var out Port
	err := db.QueryRow(
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

// ListPorts returns ports for a host ordered by port_number then protocol.
func (db *DB) ListPorts(hostID int64) ([]Port, error) {
	rows, err := db.Query(
		`SELECT id, host_id, port_number, protocol, state, service, version, product, extra_info, work_status, script_output, notes, last_seen, created_at, updated_at
		 FROM port WHERE host_id = ? ORDER BY port_number, protocol`,
		hostID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ports: %w", err)
	}
	defer rows.Close()

	var ports []Port
	for rows.Next() {
		var p Port
		if err := rows.Scan(&p.ID, &p.HostID, &p.PortNumber, &p.Protocol, &p.State, &p.Service, &p.Version, &p.Product, &p.ExtraInfo, &p.WorkStatus, &p.ScriptOutput, &p.Notes, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan port: %w", err)
		}
		ports = append(ports, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ports, nil
}

// DeletePort removes a port by ID.
func (db *DB) DeletePort(id int64) error {
	res, err := db.Exec(`DELETE FROM port WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete port: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetPortByID fetches a port by ID.
func (db *DB) GetPortByID(id int64) (Port, bool, error) {
	var p Port
	err := db.QueryRow(
		`SELECT id, host_id, port_number, protocol, state, service, version, product, extra_info, work_status, script_output, notes, last_seen, created_at, updated_at
		 FROM port WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.HostID, &p.PortNumber, &p.Protocol, &p.State, &p.Service, &p.Version, &p.Product, &p.ExtraInfo, &p.WorkStatus, &p.ScriptOutput, &p.Notes, &p.LastSeen, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Port{}, false, nil
		}
		return Port{}, false, fmt.Errorf("get port by id: %w", err)
	}
	return p, true, nil
}

// GetPortByKey fetches a port by host/number/protocol.
func (db *DB) GetPortByKey(hostID int64, portNumber int, protocol string) (Port, bool, error) {
	var p Port
	err := db.QueryRow(
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

// UpdateWorkStatus sets work_status for a single port id.
func (db *DB) UpdateWorkStatus(portID int64, status string) error {
	_, err := db.Exec(`UPDATE port SET work_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, portID)
	if err != nil {
		return fmt.Errorf("update work_status: %w", err)
	}
	return nil
}

// UpdatePortNotes updates the notes for a port.
func (db *DB) UpdatePortNotes(portID int64, notes string) error {
	_, err := db.Exec(`UPDATE port SET notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, notes, portID)
	if err != nil {
		return fmt.Errorf("update port notes: %w", err)
	}
	return nil
}

// BulkUpdateByHost sets work_status for all ports on a host in a transaction.
func (db *DB) BulkUpdateByHost(hostID int64, status string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(`UPDATE port SET work_status = ?, updated_at = CURRENT_TIMESTAMP WHERE host_id = ?`, status, hostID); err != nil {
		tx.Rollback()
		return fmt.Errorf("bulk update host: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateOpenByHost sets work_status for open ports on a host.
func (db *DB) BulkUpdateOpenByHost(hostID int64, status string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(`UPDATE port SET work_status = ?, updated_at = CURRENT_TIMESTAMP WHERE host_id = ? AND state = 'open'`, status, hostID); err != nil {
		tx.Rollback()
		return fmt.Errorf("bulk update open host: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateByPortNumber sets work_status for all ports with a given number across a project.
func (db *DB) BulkUpdateByPortNumber(projectID int64, portNumber int, status string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	_, err = tx.Exec(
		`UPDATE port SET work_status = ?, updated_at = CURRENT_TIMESTAMP WHERE port_number = ? AND host_id IN (SELECT id FROM host WHERE project_id = ?)`,
		status, portNumber, projectID,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("bulk update port number: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateOpenByPortNumber sets work_status for open ports with a given number across a project.
func (db *DB) BulkUpdateOpenByPortNumber(projectID int64, portNumber int, status string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	_, err = tx.Exec(
		`UPDATE port
		    SET work_status = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE state = 'open'
		    AND port_number = ?
		    AND host_id IN (SELECT id FROM host WHERE project_id = ?)`,
		status, portNumber, projectID,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("bulk update open port number: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateByFilter sets work_status for ports that match provided filters (protocol optional).
func (db *DB) BulkUpdateByFilter(projectID int64, hostIDs []int64, portNumbers []int, protocols []string, status string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var conditions []string
	var args []any

	conditions = append(conditions, "host_id IN (SELECT id FROM host WHERE project_id = ?)")
	args = append(args, projectID)

	if len(hostIDs) > 0 {
		placeholders := makePlaceholders(len(hostIDs))
		conditions = append(conditions, fmt.Sprintf("host_id IN (%s)", placeholders))
		for _, id := range hostIDs {
			args = append(args, id)
		}
	}
	if len(portNumbers) > 0 {
		placeholders := makePlaceholders(len(portNumbers))
		conditions = append(conditions, fmt.Sprintf("port_number IN (%s)", placeholders))
		for _, p := range portNumbers {
			args = append(args, p)
		}
	}
	if len(protocols) > 0 {
		placeholders := makePlaceholders(len(protocols))
		conditions = append(conditions, fmt.Sprintf("protocol IN (%s)", placeholders))
		for _, proto := range protocols {
			args = append(args, proto)
		}
	}

	where := strings.Join(conditions, " AND ")
	args = append([]any{status}, args...)
	query := fmt.Sprintf(`UPDATE port SET work_status = ?, updated_at = CURRENT_TIMESTAMP WHERE %s`, where)
	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("bulk update filter: %w", err)
	}
	return tx.Commit()
}

// BulkUpdateOpenByHostIDs sets work_status for open ports on a set of hosts within a project.
func (db *DB) BulkUpdateOpenByHostIDs(projectID int64, hostIDs []int64, status string) error {
	if len(hostIDs) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	placeholders := makePlaceholders(len(hostIDs))
	args := []any{status, projectID}
	for _, id := range hostIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`UPDATE port
		    SET work_status = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE state = 'open'
		    AND host_id IN (SELECT id FROM host WHERE project_id = ?)
		    AND host_id IN (%s)`,
		placeholders,
	)
	if _, err := tx.Exec(query, args...); err != nil {
		tx.Rollback()
		return fmt.Errorf("bulk update open host ids: %w", err)
	}
	return tx.Commit()
}

func makePlaceholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}
