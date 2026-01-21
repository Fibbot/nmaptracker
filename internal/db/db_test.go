package db

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sloppy/nmaptracker/internal/testutil"
)

func TestMigrationsCreateSchema(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	wantTables := map[string]struct{}{
		"project":          {},
		"scope_definition": {},
		"scan_import":      {},
		"host":             {},
		"port":             {},
	}
	tables := mustListStrings(t, db, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';`)
	for name := range wantTables {
		if _, ok := tables[name]; !ok {
			t.Fatalf("expected table %q to exist, got tables: %v", name, keys(tables))
		}
	}

	wantIndexes := map[string]struct{}{
		"idx_host_project":  {},
		"idx_host_ip":       {},
		"idx_host_in_scope": {},
		"idx_port_host":     {},
		"idx_port_status":   {},
		"idx_port_number":   {},
		"idx_port_protocol": {},
		"idx_scope_project": {},
	}
	indexes := mustListStrings(t, db, `SELECT name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%';`)
	for name := range wantIndexes {
		if _, ok := indexes[name]; !ok {
			t.Fatalf("expected index %q to exist, got indexes: %v", name, keys(indexes))
		}
	}

	if !hasUniqueIndex(t, db, "host") {
		t.Fatalf("expected unique index on host(project_id, ip_address)")
	}
	if !hasUniqueIndex(t, db, "port") {
		t.Fatalf("expected unique index on port(host_id, port_number, protocol)")
	}
	if !hasCascadeForeignKey(t, db, "host", "project") {
		t.Fatalf("expected host to reference project with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "port", "host") {
		t.Fatalf("expected port to reference host with ON DELETE CASCADE")
	}
}

func TestOpenEnablesWALAndAllowsConcurrentOpens(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "test.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	defer db1.Close()

	if mode := pragmaString(t, db1, "PRAGMA journal_mode;"); mode != "wal" {
		t.Fatalf("expected journal_mode wal, got %q", mode)
	}

	if _, err := db1.Exec(`INSERT INTO project (name) VALUES (?)`, "proj1"); err != nil {
		t.Fatalf("insert via db1: %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()

	if _, err := db2.Exec(`INSERT INTO project (name) VALUES (?)`, "proj2"); err != nil {
		t.Fatalf("insert via db2: %v", err)
	}

	var count int
	if err := db1.QueryRow(`SELECT COUNT(*) FROM project`).Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 projects, got %d", count)
	}
}

// Helpers

func mustListStrings(t *testing.T, db *DB, query string) map[string]struct{} {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()

	result := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan name: %v", err)
		}
		result[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return result
}

func pragmaString(t *testing.T, db *DB, pragma string) string {
	t.Helper()
	var val string
	if err := db.QueryRow(pragma).Scan(&val); err != nil {
		t.Fatalf("pragma query %q: %v", pragma, err)
	}
	return val
}

func hasUniqueIndex(t *testing.T, db *DB, table string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf(`PRAGMA index_list(%s);`, table))
	if err != nil {
		t.Fatalf("index_list %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list: %v", err)
		}
		if unique == 1 {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return false
}

func hasCascadeForeignKey(t *testing.T, db *DB, table, ref string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf(`PRAGMA foreign_key_list(%s);`, table))
	if err != nil {
		t.Fatalf("foreign_key_list %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, seq                                       int
			refTable, from, to, onUpdate, onDelete, match string
		)
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan foreign_key_list: %v", err)
		}
		if refTable == ref && strings.EqualFold(onDelete, "CASCADE") {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return false
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
