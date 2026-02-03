package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/testutil"
)

func TestProjectsCLI(t *testing.T) {
	tmp := testutil.TempDir(t)
	dbPath := filepath.Join(tmp, "cli.db")

	exit := run([]string{"nmap-tracker", "projects", "create", "CLI Project", "--db", dbPath}, ioDiscard{}, ioDiscard{})
	if exit != 0 {
		t.Fatalf("projects create exit %d", exit)
	}

	var stdout bytes.Buffer
	exit = run([]string{"nmap-tracker", "projects", "list", "--db", dbPath}, &stdout, ioDiscard{})
	if exit != 0 {
		t.Fatalf("projects list exit %d", exit)
	}
	if !strings.Contains(stdout.String(), "CLI Project") {
		t.Fatalf("expected project in list output, got %q", stdout.String())
	}
}

func TestImportCLI(t *testing.T) {
	tmp := testutil.TempDir(t)
	dbPath := filepath.Join(tmp, "cli.db")

	// Create project
	exit := run([]string{"nmap-tracker", "projects", "create", "ImportProj", "--db", dbPath}, ioDiscard{}, ioDiscard{})
	if exit != 0 {
		t.Fatalf("projects create exit %d", exit)
	}

	// Locate sampleNmap1.xml fixture at repo root.
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	samplePath := filepath.Join(root, "sampleNmap1.xml")
	if _, err := os.Stat(samplePath); err != nil {
		t.Fatalf("sample fixture missing: %v", err)
	}

	exit = run([]string{"nmap-tracker", "import", "--project", "ImportProj", "--db", dbPath, samplePath}, ioDiscard{}, ioDiscard{})
	if exit != 0 {
		t.Fatalf("import exit %d", exit)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	var hostCount, portCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM host`).Scan(&hostCount); err != nil {
		t.Fatalf("count hosts: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM port`).Scan(&portCount); err != nil {
		t.Fatalf("count ports: %v", err)
	}
	if hostCount != 1 {
		t.Fatalf("expected 1 host, got %d", hostCount)
	}
	if portCount != 25 {
		t.Fatalf("expected 25 ports, got %d", portCount)
	}
}

// ioDiscard is a minimal io.Writer to drop output without importing io once more.
type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
