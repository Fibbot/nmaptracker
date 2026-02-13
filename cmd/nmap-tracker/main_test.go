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
	if portCount != 9 {
		t.Fatalf("expected 9 ports, got %d", portCount)
	}
}

func TestImportCLIWithManualSourceMetadataFlags(t *testing.T) {
	tmp := testutil.TempDir(t)
	dbPath := filepath.Join(tmp, "cli.db")

	exit := run([]string{"nmap-tracker", "projects", "create", "ImportMetaProj", "--db", dbPath}, ioDiscard{}, ioDiscard{})
	if exit != 0 {
		t.Fatalf("projects create exit %d", exit)
	}

	xmlPath := filepath.Join(tmp, "manual_source.xml")
	xmlContent := `<?xml version="1.0"?>
<nmaprun args="nmap -sV 198.51.100.20">
  <host>
    <status state="up"/>
    <address addr="198.51.100.20" addrtype="ipv4"/>
  </host>
</nmaprun>`
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0o600); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	exit = run(
		[]string{
			"nmap-tracker", "import",
			"--project", "ImportMetaProj",
			"--scanner-label", "cli-scanner",
			"--source-ip", "192.0.2.77",
			"--source-port", "4444",
			"--db", dbPath,
			xmlPath,
		},
		ioDiscard{},
		ioDiscard{},
	)
	if exit != 0 {
		t.Fatalf("import exit %d", exit)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	project, found, err := database.GetProjectByName("ImportMetaProj")
	if err != nil || !found {
		t.Fatalf("find project: err=%v found=%v", err, found)
	}
	imports, err := database.ListScanImports(project.ID)
	if err != nil {
		t.Fatalf("list imports: %v", err)
	}
	if len(imports) != 1 {
		t.Fatalf("expected one import, got %d", len(imports))
	}
	if imports[0].ScannerLabel != "cli-scanner" {
		t.Fatalf("unexpected scanner_label: %q", imports[0].ScannerLabel)
	}
	if imports[0].SourceIP == nil || *imports[0].SourceIP != "192.0.2.77" {
		t.Fatalf("unexpected source_ip: %v", imports[0].SourceIP)
	}
	if imports[0].SourcePort == nil || *imports[0].SourcePort != 4444 {
		t.Fatalf("unexpected source_port: %v", imports[0].SourcePort)
	}
}

func TestImportCLIRejectsInvalidSourceIPFlag(t *testing.T) {
	tmp := testutil.TempDir(t)
	dbPath := filepath.Join(tmp, "cli.db")

	exit := run([]string{"nmap-tracker", "projects", "create", "ImportInvalidProj", "--db", dbPath}, ioDiscard{}, ioDiscard{})
	if exit != 0 {
		t.Fatalf("projects create exit %d", exit)
	}

	xmlPath := filepath.Join(tmp, "invalid_source.xml")
	xmlContent := `<?xml version="1.0"?><nmaprun><host><address addr="198.51.100.21" addrtype="ipv4"/></host></nmaprun>`
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0o600); err != nil {
		t.Fatalf("write xml: %v", err)
	}

	var stderr bytes.Buffer
	exit = run(
		[]string{
			"nmap-tracker", "import",
			"--project", "ImportInvalidProj",
			"--source-ip", "not-an-ip",
			"--db", dbPath,
			xmlPath,
		},
		ioDiscard{},
		&stderr,
	)
	if exit == 0 {
		t.Fatalf("expected non-zero exit for invalid source ip")
	}
	if !strings.Contains(stderr.String(), "invalid source_ip") {
		t.Fatalf("expected invalid source_ip error, got %q", stderr.String())
	}
}

// ioDiscard is a minimal io.Writer to drop output without importing io once more.
type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
