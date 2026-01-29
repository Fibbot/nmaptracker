package db

import (
	"database/sql"
	"fmt"
)

// UpsertHost inserts or updates a host keyed by (project_id, ip_address).
func (db *DB) UpsertHost(h Host) (Host, error) {
	var out Host
	err := db.QueryRow(
		`INSERT INTO host (project_id, ip_address, hostname, os_guess, in_scope, notes)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, ip_address) DO UPDATE SET
		   hostname=excluded.hostname,
		   os_guess=excluded.os_guess,
		   in_scope=excluded.in_scope,
		   notes=excluded.notes,
		   updated_at=CURRENT_TIMESTAMP
		 RETURNING id, project_id, ip_address, hostname, os_guess, in_scope, notes, created_at, updated_at`,
		h.ProjectID, h.IPAddress, h.Hostname, h.OSGuess, h.InScope, h.Notes,
	).Scan(&out.ID, &out.ProjectID, &out.IPAddress, &out.Hostname, &out.OSGuess, &out.InScope, &out.Notes, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Host{}, fmt.Errorf("upsert host: %w", err)
	}
	return out, nil
}

// GetHostByIP fetches a host by project and IP.
func (db *DB) GetHostByIP(projectID int64, ip string) (Host, bool, error) {
	var h Host
	err := db.QueryRow(
		`SELECT id, project_id, ip_address, hostname, os_guess, in_scope, notes, created_at, updated_at
		 FROM host WHERE project_id = ? AND ip_address = ?`,
		projectID, ip,
	).Scan(&h.ID, &h.ProjectID, &h.IPAddress, &h.Hostname, &h.OSGuess, &h.InScope, &h.Notes, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Host{}, false, nil
		}
		return Host{}, false, fmt.Errorf("get host: %w", err)
	}
	return h, true, nil
}

// GetHostByID fetches a host by id.
func (db *DB) GetHostByID(id int64) (Host, bool, error) {
	var h Host
	err := db.QueryRow(
		`SELECT id, project_id, ip_address, hostname, os_guess, in_scope, notes, created_at, updated_at
		 FROM host WHERE id = ?`,
		id,
	).Scan(&h.ID, &h.ProjectID, &h.IPAddress, &h.Hostname, &h.OSGuess, &h.InScope, &h.Notes, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Host{}, false, nil
		}
		return Host{}, false, fmt.Errorf("get host by id: %w", err)
	}
	return h, true, nil
}

// ListHosts returns hosts for a project ordered by ip_address.
func (db *DB) ListHosts(projectID int64) ([]Host, error) {
	rows, err := db.Query(
		`SELECT id, project_id, ip_address, hostname, os_guess, in_scope, notes, created_at, updated_at
		 FROM host WHERE project_id = ? ORDER BY ip_address`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	defer rows.Close()

	var hosts []Host
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.ProjectID, &h.IPAddress, &h.Hostname, &h.OSGuess, &h.InScope, &h.Notes, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan host: %w", err)
		}
		hosts = append(hosts, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

// DeleteHost removes a host by ID.
func (db *DB) DeleteHost(id int64) error {
	res, err := db.Exec(`DELETE FROM host WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete host: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateHostNotes updates the notes for a host.
func (db *DB) UpdateHostNotes(id int64, notes string) error {
	_, err := db.Exec(`UPDATE host SET notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, notes, id)
	if err != nil {
		return fmt.Errorf("update host notes: %w", err)
	}
	return nil
}

// UpdateHostScope updates the in_scope status for a host.
func (db *DB) UpdateHostScope(id int64, inScope bool) error {
	_, err := db.Exec(`UPDATE host SET in_scope = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, inScope, id)
	if err != nil {
		return fmt.Errorf("update host scope: %w", err)
	}
	return nil
}
