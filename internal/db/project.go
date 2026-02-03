package db

import (
	"database/sql"
	"fmt"
)

// CreateProject inserts a new project.
func (db *DB) CreateProject(name string) (Project, error) {
	var p Project
	err := db.QueryRow(
		`INSERT INTO project (name) VALUES (?) RETURNING id, name, created_at, updated_at`,
		name,
	).Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}
	return p, nil
}

// UpdateProject updates an existing project's name.
func (db *DB) UpdateProject(id int64, name string) error {
	res, err := db.Exec(`UPDATE project SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListProjects returns all projects ordered by name.
func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Query(`SELECT id, name, created_at, updated_at FROM project ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return projects, nil
}

// DeleteProject removes a project by ID.
func (db *DB) DeleteProject(id int64) error {
	res, err := db.Exec(`DELETE FROM project WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetProjectByName returns a project by exact name.
func (db *DB) GetProjectByName(name string) (Project, bool, error) {
	var p Project
	err := db.QueryRow(`SELECT id, name, created_at, updated_at FROM project WHERE name = ?`, name).
		Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Project{}, false, nil
		}
		return Project{}, false, fmt.Errorf("get project by name: %w", err)
	}
	return p, true, nil
}

// GetProjectByID returns a project by ID.
func (db *DB) GetProjectByID(id int64) (Project, bool, error) {
	var p Project
	err := db.QueryRow(`SELECT id, name, created_at, updated_at FROM project WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Project{}, false, nil
		}
		return Project{}, false, fmt.Errorf("get project by id: %w", err)
	}
	return p, true, nil
}
