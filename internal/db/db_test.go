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
		"project":                 {},
		"scope_definition":        {},
		"scan_import":             {},
		"scan_import_intent":      {},
		"host":                    {},
		"host_observation":        {},
		"port":                    {},
		"port_observation":        {},
		"expected_asset_baseline": {},
	}
	tables := mustListStrings(t, db, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';`)
	for name := range wantTables {
		if _, ok := tables[name]; !ok {
			t.Fatalf("expected table %q to exist, got tables: %v", name, keys(tables))
		}
	}

	wantIndexes := map[string]struct{}{
		"idx_host_project":                    {},
		"idx_host_ip":                         {},
		"idx_host_in_scope":                   {},
		"idx_host_ip_int":                     {},
		"idx_port_host":                       {},
		"idx_port_status":                     {},
		"idx_port_number":                     {},
		"idx_port_protocol":                   {},
		"idx_scope_project":                   {},
		"idx_scan_import_intent_scan_import":  {},
		"idx_scan_import_intent_intent":       {},
		"idx_host_observation_project":        {},
		"idx_host_observation_project_ip":     {},
		"idx_host_observation_import":         {},
		"idx_port_observation_project":        {},
		"idx_port_observation_project_ip":     {},
		"idx_port_observation_import":         {},
		"idx_port_observation_open":           {},
		"idx_expected_asset_baseline_project": {},
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
	if !hasUniqueIndex(t, db, "scan_import_intent") {
		t.Fatalf("expected unique index on scan_import_intent(scan_import_id, intent)")
	}
	if !hasUniqueIndex(t, db, "host_observation") {
		t.Fatalf("expected unique index on host_observation(scan_import_id, ip_address)")
	}
	if !hasUniqueIndex(t, db, "port_observation") {
		t.Fatalf("expected unique index on port_observation(scan_import_id, ip_address, port_number, protocol)")
	}
	if !hasUniqueIndex(t, db, "expected_asset_baseline") {
		t.Fatalf("expected unique index on expected_asset_baseline(project_id, definition)")
	}
	if !hasCascadeForeignKey(t, db, "host", "project") {
		t.Fatalf("expected host to reference project with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "port", "host") {
		t.Fatalf("expected port to reference host with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "scan_import_intent", "scan_import") {
		t.Fatalf("expected scan_import_intent to reference scan_import with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "host_observation", "scan_import") {
		t.Fatalf("expected host_observation to reference scan_import with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "host_observation", "project") {
		t.Fatalf("expected host_observation to reference project with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "port_observation", "scan_import") {
		t.Fatalf("expected port_observation to reference scan_import with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "port_observation", "project") {
		t.Fatalf("expected port_observation to reference project with ON DELETE CASCADE")
	}
	if !hasCascadeForeignKey(t, db, "expected_asset_baseline", "project") {
		t.Fatalf("expected expected_asset_baseline to reference project with ON DELETE CASCADE")
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

func TestMigrationsConvertParkingLotToFlagged(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "migration-reapply.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	project, err := db.CreateProject("migration-check")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := db.UpsertHost(Host{
		ProjectID: project.ID,
		IPAddress: "192.0.2.45",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	if _, err := db.UpsertPort(Port{
		HostID:     host.ID,
		PortNumber: 445,
		Protocol:   "tcp",
		State:      "open",
		Service:    "microsoft-ds",
		WorkStatus: "parking_lot",
	}); err != nil {
		t.Fatalf("upsert parking_lot port: %v", err)
	}

	var before string
	if err := db.QueryRow(`SELECT work_status FROM port WHERE host_id = ? LIMIT 1`, host.ID).Scan(&before); err != nil {
		t.Fatalf("query pre-migration status: %v", err)
	}
	if before != "parking_lot" {
		t.Fatalf("expected pre-migration status parking_lot, got %q", before)
	}

	if err := runMigrations(db.DB); err != nil {
		t.Fatalf("re-run migrations: %v", err)
	}

	var after string
	if err := db.QueryRow(`SELECT work_status FROM port WHERE host_id = ? LIMIT 1`, host.ID).Scan(&after); err != nil {
		t.Fatalf("query post-migration status: %v", err)
	}
	if after != "flagged" {
		t.Fatalf("expected post-migration status flagged, got %q", after)
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
