package db

import (
	"database/sql"
	"fmt"
)

// AddScopeDefinition creates a scope entry for a project.
func (db *DB) AddScopeDefinition(projectID int64, definition, typ string) (ScopeDefinition, error) {
	var s ScopeDefinition
	err := db.QueryRow(
		`INSERT INTO scope_definition (project_id, definition, type) VALUES (?, ?, ?) RETURNING id, project_id, definition, type, created_at`,
		projectID, definition, typ,
	).Scan(&s.ID, &s.ProjectID, &s.Definition, &s.Type, &s.CreatedAt)
	if err != nil {
		return ScopeDefinition{}, fmt.Errorf("insert scope_definition: %w", err)
	}
	return s, nil
}

// ListScopeDefinitions returns all scope definitions for a project ordered by id.
func (db *DB) ListScopeDefinitions(projectID int64) ([]ScopeDefinition, error) {
	rows, err := db.Query(
		`SELECT id, project_id, definition, type, created_at FROM scope_definition WHERE project_id = ? ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scope_definition: %w", err)
	}
	defer rows.Close()

	var defs []ScopeDefinition
	for rows.Next() {
		var s ScopeDefinition
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Definition, &s.Type, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan scope_definition: %w", err)
		}
		defs = append(defs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return defs, nil
}

// DeleteScopeDefinition removes a scope definition by ID.
func (db *DB) DeleteScopeDefinition(id int64) error {
	res, err := db.Exec(`DELETE FROM scope_definition WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete scope_definition: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
