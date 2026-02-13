package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sloppy/nmaptracker/internal/testutil"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := testutil.TempDir(t)
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

func TestProjectCRUD(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p1, err := db.CreateProject("Alpha")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	p2, err := db.CreateProject("Beta")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Name != "Alpha" || projects[1].Name != "Beta" {
		t.Fatalf("projects not sorted by name: %#v", projects)
	}

	if err := db.DeleteProject(p1.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	projects, err = db.ListProjects()
	if err != nil {
		t.Fatalf("list projects after delete: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != p2.ID {
		t.Fatalf("unexpected projects after delete: %#v", projects)
	}
}

func TestScopeCRUD(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p, err := db.CreateProject("scope-proj")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	s1, err := db.AddScopeDefinition(p.ID, "10.0.0.0/24", "cidr")
	if err != nil {
		t.Fatalf("add scope1: %v", err)
	}
	s2, err := db.AddScopeDefinition(p.ID, "10.0.0.5", "ip")
	if err != nil {
		t.Fatalf("add scope2: %v", err)
	}

	defs, err := db.ListScopeDefinitions(p.ID)
	if err != nil {
		t.Fatalf("list scopes: %v", err)
	}
	if len(defs) != 2 || defs[0].ID != s1.ID || defs[1].ID != s2.ID {
		t.Fatalf("unexpected scopes: %#v", defs)
	}

	if err := db.DeleteScopeDefinition(s1.ID); err != nil {
		t.Fatalf("delete scope: %v", err)
	}
	defs, err = db.ListScopeDefinitions(p.ID)
	if err != nil {
		t.Fatalf("list scopes after delete: %v", err)
	}
	if len(defs) != 1 || defs[0].ID != s2.ID {
		t.Fatalf("unexpected scopes after delete: %#v", defs)
	}
}

func TestHostUpsertAndList(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p, err := db.CreateProject("hosts")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	h, err := db.UpsertHost(Host{
		ProjectID: p.ID,
		IPAddress: "192.0.2.1",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("insert host: %v", err)
	}

	h2, err := db.UpsertHost(Host{
		ProjectID: p.ID,
		IPAddress: "192.0.2.1",
		Hostname:  "example",
		OSGuess:   "linux",
		InScope:   false,
		Notes:     "updated",
	})
	if err != nil {
		t.Fatalf("upsert host update: %v", err)
	}
	if h.ID != h2.ID {
		t.Fatalf("expected same host id on upsert, got %d and %d", h.ID, h2.ID)
	}

	hosts, err := db.ListHosts(p.ID)
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	got := hosts[0]
	if got.Hostname != "example" || got.OSGuess != "linux" || got.InScope != false || got.Notes != "updated" {
		t.Fatalf("host not updated: %#v", got)
	}
	if got.LatestScan != HostLatestScanNone {
		t.Fatalf("expected default latest_scan=%q, got %q", HostLatestScanNone, got.LatestScan)
	}
}

func TestUpdateHostLatestScan(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p, err := db.CreateProject("host-scan")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	h, err := db.UpsertHost(Host{ProjectID: p.ID, IPAddress: "192.0.2.25", InScope: true})
	if err != nil {
		t.Fatalf("insert host: %v", err)
	}

	if err := db.UpdateHostLatestScan(h.ID, "top_1k_tcp"); err != nil {
		t.Fatalf("update host latest scan: %v", err)
	}

	updated, found, err := db.GetHostByID(h.ID)
	if err != nil || !found {
		t.Fatalf("get host by id: err=%v found=%v", err, found)
	}
	if updated.LatestScan != HostLatestScanTop1K {
		t.Fatalf("expected latest_scan=%q, got %q", HostLatestScanTop1K, updated.LatestScan)
	}

	if err := db.UpdateHostLatestScan(h.ID, "invalid"); err == nil {
		t.Fatalf("expected invalid latest scan update error")
	}
}

func TestPortUpsertAndList(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p, _ := db.CreateProject("ports")
	h, _ := db.UpsertHost(Host{ProjectID: p.ID, IPAddress: "203.0.113.5", InScope: true})

	now := time.Now().UTC().Truncate(time.Second)
	port, err := db.UpsertPort(Port{
		HostID:     h.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "open",
		Service:    "http",
		WorkStatus: "scanned",
		LastSeen:   now,
	})
	if err != nil {
		t.Fatalf("insert port: %v", err)
	}

	updated, err := db.UpsertPort(Port{
		HostID:     h.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "filtered",
		Service:    "https",
		WorkStatus: "flagged",
		// LastSeen zero ensures we keep previous value via COALESCE.
	})
	if err != nil {
		t.Fatalf("update port: %v", err)
	}
	if updated.ID != port.ID {
		t.Fatalf("expected same port id on upsert, got %d and %d", updated.ID, port.ID)
	}
	if !updated.LastSeen.Equal(now) {
		t.Fatalf("last_seen should remain unchanged when not provided, got %v want %v", updated.LastSeen, now)
	}
	if updated.Service != "https" || updated.State != "filtered" || updated.WorkStatus != "flagged" {
		t.Fatalf("port fields not updated: %#v", updated)
	}

	// Ensure protocol-aware uniqueness allows TCP+UDP same port number.
	udp, err := db.UpsertPort(Port{
		HostID:     h.ID,
		PortNumber: 80,
		Protocol:   "udp",
		State:      "open",
		Service:    "http-udp",
		WorkStatus: "scanned",
	})
	if err != nil {
		t.Fatalf("insert udp port: %v", err)
	}
	if udp.Protocol != "udp" {
		t.Fatalf("expected udp protocol, got %s", udp.Protocol)
	}

	ports, err := db.ListPorts(h.ID)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}
}

func TestInsertScanImport(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	p, _ := db.CreateProject("imports")
	sourceIP := "192.0.2.15"
	sourcePort := 4444
	sourcePortRaw := "raw-token"
	record, err := db.InsertScanImport(ScanImport{
		ProjectID:     p.ID,
		Filename:      "scan.xml",
		HostsFound:    3,
		PortsFound:    5,
		NmapArgs:      "nmap -S 192.0.2.15 --source-port 4444 198.51.100.10",
		ScannerLabel:  "scanner-1",
		SourceIP:      &sourceIP,
		SourcePort:    &sourcePort,
		SourcePortRaw: &sourcePortRaw,
	})
	if err != nil {
		t.Fatalf("insert scan import: %v", err)
	}
	if record.ID == 0 || record.ProjectID != p.ID || record.Filename != "scan.xml" || record.HostsFound != 3 || record.PortsFound != 5 {
		t.Fatalf("unexpected scan import record: %#v", record)
	}
	if record.NmapArgs == "" || record.ScannerLabel != "scanner-1" {
		t.Fatalf("expected metadata fields to be persisted, got %#v", record)
	}
	if record.SourceIP == nil || *record.SourceIP != sourceIP {
		t.Fatalf("expected source_ip %q, got %v", sourceIP, record.SourceIP)
	}
	if record.SourcePort == nil || *record.SourcePort != sourcePort {
		t.Fatalf("expected source_port %d, got %v", sourcePort, record.SourcePort)
	}
	if record.SourcePortRaw == nil || *record.SourcePortRaw != sourcePortRaw {
		t.Fatalf("expected source_port_raw %q, got %v", sourcePortRaw, record.SourcePortRaw)
	}
	if record.ImportTime.IsZero() {
		t.Fatalf("expected import_time to be set")
	}
}

func TestSetScanImportIntentsSyncsHostLatestScan(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("intent-sync")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := db.UpsertHost(Host{
		ProjectID: project.ID,
		IPAddress: "198.51.100.42",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	olderImport, err := db.InsertScanImport(ScanImport{ProjectID: project.ID, Filename: "older.xml"})
	if err != nil {
		t.Fatalf("insert older import: %v", err)
	}
	newerImport, err := db.InsertScanImport(ScanImport{ProjectID: project.ID, Filename: "newer.xml"})
	if err != nil {
		t.Fatalf("insert newer import: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.InsertHostObservation(HostObservation{
		ScanImportID: olderImport.ID,
		ProjectID:    project.ID,
		IPAddress:    host.IPAddress,
		Hostname:     "",
		InScope:      true,
		HostState:    "up",
	}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert older host observation: %v", err)
	}
	if _, err := tx.InsertHostObservation(HostObservation{
		ScanImportID: newerImport.ID,
		ProjectID:    project.ID,
		IPAddress:    host.IPAddress,
		Hostname:     "",
		InScope:      true,
		HostState:    "up",
	}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert newer host observation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit observations: %v", err)
	}

	if err := db.SetScanImportIntents(project.ID, newerImport.ID, []ScanImportIntentInput{
		{Intent: IntentTop1KTCP, Source: IntentSourceManual, Confidence: 1.0},
	}); err != nil {
		t.Fatalf("set newer import intents: %v", err)
	}

	updated, found, err := db.GetHostByID(host.ID)
	if err != nil || !found {
		t.Fatalf("get host after newer intents: err=%v found=%v", err, found)
	}
	if updated.LatestScan != HostLatestScanTop1K {
		t.Fatalf("expected latest_scan %q, got %q", HostLatestScanTop1K, updated.LatestScan)
	}

	// Updating an older import should not overwrite the latest observation's classification.
	if err := db.SetScanImportIntents(project.ID, olderImport.ID, []ScanImportIntentInput{
		{Intent: IntentPingSweep, Source: IntentSourceManual, Confidence: 1.0},
	}); err != nil {
		t.Fatalf("set older import intents: %v", err)
	}

	updated, found, err = db.GetHostByID(host.ID)
	if err != nil || !found {
		t.Fatalf("get host after older intents: err=%v found=%v", err, found)
	}
	if updated.LatestScan != HostLatestScanTop1K {
		t.Fatalf("expected latest_scan to remain %q, got %q", HostLatestScanTop1K, updated.LatestScan)
	}
}
