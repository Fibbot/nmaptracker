package db

import (
	"database/sql"
	"fmt"
	"net/netip"
	"strings"
)

// ListExpectedAssetBaselines returns baselines for a project.
func (db *DB) ListExpectedAssetBaselines(projectID int64) ([]ExpectedAssetBaseline, error) {
	rows, err := db.Query(
		`SELECT id, project_id, definition, type, created_at
		   FROM expected_asset_baseline
		  WHERE project_id = ?
		  ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list expected asset baselines: %w", err)
	}
	defer rows.Close()

	var items []ExpectedAssetBaseline
	for rows.Next() {
		var item ExpectedAssetBaseline
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Definition, &item.Type, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan expected asset baseline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expected asset baselines rows: %w", err)
	}
	return items, nil
}

// BulkAddExpectedAssetBaselines inserts baseline definitions for a project.
func (db *DB) BulkAddExpectedAssetBaselines(projectID int64, defs []string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO expected_asset_baseline (project_id, definition, type) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare baseline insert: %w", err)
	}
	defer stmt.Close()

	added := 0
	for _, raw := range defs {
		def := strings.TrimSpace(raw)
		if def == "" {
			continue
		}
		res, err := stmt.Exec(projectID, def, baselineType(def))
		if err != nil {
			return added, fmt.Errorf("insert expected asset baseline: %w", err)
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			added++
		}
	}

	if err := tx.Commit(); err != nil {
		return added, fmt.Errorf("commit baseline insert: %w", err)
	}
	return added, nil
}

// DeleteExpectedAssetBaseline removes one baseline, scoped by project.
func (db *DB) DeleteExpectedAssetBaseline(projectID, baselineID int64) error {
	res, err := db.Exec(`DELETE FROM expected_asset_baseline WHERE id = ? AND project_id = ?`, baselineID, projectID)
	if err != nil {
		return fmt.Errorf("delete expected asset baseline: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func baselineType(definition string) string {
	if _, err := netip.ParsePrefix(definition); err == nil {
		return "cidr"
	}
	if _, err := netip.ParseAddr(definition); err == nil {
		return "ip"
	}
	return "unknown"
}
