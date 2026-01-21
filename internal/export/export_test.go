package export

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/testutil"
)

func TestExportProjectJSON(t *testing.T) {
	database := setupExportDB(t)
	defer database.Close()

	var buf bytes.Buffer
	if err := ExportProjectJSON(database, 1, &buf); err != nil {
		t.Fatalf("export json: %v", err)
	}

	expected := readFixture(t, "project.json")
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(expected) {
		t.Fatalf("json export mismatch\nexpected:\n%s\n\ngot:\n%s", expected, buf.String())
	}
}

func TestExportProjectCSV(t *testing.T) {
	database := setupExportDB(t)
	defer database.Close()

	var buf bytes.Buffer
	if err := ExportProjectCSV(database, 1, &buf); err != nil {
		t.Fatalf("export csv: %v", err)
	}

	expected := readFixture(t, "project.csv")
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(expected) {
		t.Fatalf("csv export mismatch\nexpected:\n%s\n\ngot:\n%s", expected, buf.String())
	}
}

func TestExportHostJSON(t *testing.T) {
	database := setupExportDB(t)
	defer database.Close()

	var buf bytes.Buffer
	if err := ExportHostJSON(database, 1, 1, &buf); err != nil {
		t.Fatalf("export host json: %v", err)
	}

	expected := readFixture(t, "host.json")
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(expected) {
		t.Fatalf("host json export mismatch\nexpected:\n%s\n\ngot:\n%s", expected, buf.String())
	}
}

func TestExportHostCSV(t *testing.T) {
	database := setupExportDB(t)
	defer database.Close()

	var buf bytes.Buffer
	if err := ExportHostCSV(database, 1, 1, &buf); err != nil {
		t.Fatalf("export host csv: %v", err)
	}

	expected := readFixture(t, "host.csv")
	if strings.TrimSpace(buf.String()) != strings.TrimSpace(expected) {
		t.Fatalf("host csv export mismatch\nexpected:\n%s\n\ngot:\n%s", expected, buf.String())
	}
}

func TestExportProjectCSVHeader(t *testing.T) {
	database := setupExportDB(t)
	defer database.Close()

	var buf bytes.Buffer
	if err := ExportProjectCSV(database, 1, &buf); err != nil {
		t.Fatalf("export csv: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected csv output")
	}
	expectedHeader := "project_id,project_name,host_id,ip_address,hostname,os_guess,in_scope,host_notes,port_id,port_number,protocol,state,service,version,product,extra_info,work_status,script_output,port_notes,last_seen"
	if lines[0] != expectedHeader {
		t.Fatalf("unexpected csv header: %s", lines[0])
	}
}

func setupExportDB(t *testing.T) *db.DB {
	t.Helper()
	dir := testutil.TempDir(t)
	database, err := db.Open(filepath.Join(dir, "export.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	project, err := database.CreateProject("Acme")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := database.AddScopeDefinition(project.ID, "10.0.0.0/24", "include"); err != nil {
		t.Fatalf("add scope: %v", err)
	}
	if _, err := database.AddScopeDefinition(project.ID, "10.0.0.5", "exclude"); err != nil {
		t.Fatalf("add scope: %v", err)
	}

	if _, err := database.InsertScanImport(db.ScanImport{
		ProjectID:  project.ID,
		Filename:   "scan.xml",
		HostsFound: 2,
		PortsFound: 3,
	}); err != nil {
		t.Fatalf("insert scan import: %v", err)
	}

	hostA, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.0.10",
		Hostname:  "web-01",
		OSGuess:   "Linux",
		InScope:   true,
		Notes:     "first host",
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.0.20",
		Hostname:  "dns-01",
		OSGuess:   "FreeBSD",
		InScope:   true,
		Notes:     "dns host",
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	portA, err := database.UpsertPort(db.Port{
		HostID:       hostA.ID,
		PortNumber:   22,
		Protocol:     "tcp",
		State:        "open",
		Service:      "ssh",
		Version:      "OpenSSH 8.2",
		Product:      "OpenSSH",
		ExtraInfo:    "Ubuntu",
		WorkStatus:   "flagged",
		ScriptOutput: "ssh-hostkey: example",
		Notes:        "check ssh",
		LastSeen:     time.Date(2024, 1, 4, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	portB, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "closed",
		WorkStatus: "scanned",
		LastSeen:   time.Date(2024, 1, 4, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	portC, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 53,
		Protocol:   "udp",
		State:      "open",
		Service:    "domain",
		WorkStatus: "in_progress",
		LastSeen:   time.Date(2024, 1, 5, 4, 5, 6, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	setFixedTimes(t, database, project.ID, hostA.ID, hostB.ID, portA.ID, portB.ID, portC.ID)

	return database
}

func setFixedTimes(t *testing.T, database *db.DB, projectID, hostAID, hostBID, portAID, portBID, portCID int64) {
	t.Helper()
	projectTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	hostTime := time.Date(2024, 1, 2, 4, 5, 6, 0, time.UTC)
	portTime := time.Date(2024, 1, 2, 5, 6, 7, 0, time.UTC)
	scopeTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	importTime := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	if _, err := database.Exec(`UPDATE project SET created_at = ?, updated_at = ? WHERE id = ?`, toDBTime(projectTime), toDBTime(projectTime), projectID); err != nil {
		t.Fatalf("update project time: %v", err)
	}
	if _, err := database.Exec(`UPDATE scope_definition SET created_at = ?`, toDBTime(scopeTime)); err != nil {
		t.Fatalf("update scope time: %v", err)
	}
	if _, err := database.Exec(`UPDATE scan_import SET import_time = ?`, toDBTime(importTime)); err != nil {
		t.Fatalf("update scan import time: %v", err)
	}
	if _, err := database.Exec(`UPDATE host SET created_at = ?, updated_at = ? WHERE id IN (?, ?)`, toDBTime(hostTime), toDBTime(hostTime), hostAID, hostBID); err != nil {
		t.Fatalf("update host time: %v", err)
	}
	if _, err := database.Exec(`UPDATE port SET created_at = ?, updated_at = ? WHERE id IN (?, ?, ?)`, toDBTime(portTime), toDBTime(portTime), portAID, portBID, portCID); err != nil {
		t.Fatalf("update port time: %v", err)
	}
	if _, err := database.Exec(`UPDATE port SET work_status = ?, script_output = ? WHERE id = ?`, "flagged", "ssh-hostkey: example", portAID); err != nil {
		t.Fatalf("update port status: %v", err)
	}
	if _, err := database.Exec(`UPDATE port SET work_status = ?, script_output = ? WHERE id = ?`, "scanned", "", portBID); err != nil {
		t.Fatalf("update port status: %v", err)
	}
	if _, err := database.Exec(`UPDATE port SET work_status = ?, script_output = ? WHERE id = ?`, "in_progress", "", portCID); err != nil {
		t.Fatalf("update port status: %v", err)
	}
}

func toDBTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05")
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(content)
}
