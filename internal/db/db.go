package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// DB wraps sql.DB for future expansion.
type DB struct {
	*sql.DB
}

// Open opens (or creates) a SQLite database at the given path, enables WAL and
// foreign keys, and runs embedded migrations in order.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s", path)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := sqlDB.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := sqlDB.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if err := runMigrations(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	if err := ensureHostIPIntIndex(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	if err := backfillHostIPInt(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return &DB{sqlDB}, nil
}

func runMigrations(sqlDB *sql.DB) error {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	// Ensure deterministic order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := strings.TrimSpace(string(content))
		if sqlText == "" {
			continue
		}
		if _, err := sqlDB.Exec(sqlText); err != nil {
			if isDuplicateColumnError(err) {
				// A failed migration wrapped in BEGIN/COMMIT can leave an open
				// transaction, so clear it before moving to later migrations.
				_, _ = sqlDB.Exec(`ROLLBACK;`)
				continue
			}
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	return strings.Contains(err.Error(), "duplicate column name")
}

func backfillHostIPInt(sqlDB *sql.DB) error {
	exists, err := columnExists(sqlDB, "host", "ip_int")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	rows, err := sqlDB.Query(`SELECT id, ip_address FROM host WHERE ip_int IS NULL`)
	if err != nil {
		return fmt.Errorf("backfill ip_int select: %w", err)
	}
	defer rows.Close()

	type update struct {
		id    int64
		ipInt int64
	}
	var updates []update
	for rows.Next() {
		var id int64
		var ip string
		if err := rows.Scan(&id, &ip); err != nil {
			return fmt.Errorf("backfill ip_int scan: %w", err)
		}
		if ipInt, ok := ipv4ToInt(ip); ok {
			updates = append(updates, update{id: id, ipInt: ipInt})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill ip_int rows: %w", err)
	}
	if len(updates) == 0 {
		return nil
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("backfill ip_int begin: %w", err)
	}
	stmt, err := tx.Prepare(`UPDATE host SET ip_int = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("backfill ip_int prepare: %w", err)
	}
	defer stmt.Close()

	for _, upd := range updates {
		if _, err := stmt.Exec(upd.ipInt, upd.id); err != nil {
			tx.Rollback()
			return fmt.Errorf("backfill ip_int update: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("backfill ip_int commit: %w", err)
	}
	return nil
}

func ensureHostIPIntIndex(sqlDB *sql.DB) error {
	exists, err := columnExists(sqlDB, "host", "ip_int")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if _, err := sqlDB.Exec(`CREATE INDEX IF NOT EXISTS idx_host_ip_int ON host(ip_int)`); err != nil {
		return fmt.Errorf("create ip_int index: %w", err)
	}
	return nil
}

func columnExists(sqlDB *sql.DB, table, column string) (bool, error) {
	rows, err := sqlDB.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, fmt.Errorf("table info %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("table info scan: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("table info rows: %w", err)
	}
	return false, nil
}
