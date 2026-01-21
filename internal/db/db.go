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
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
