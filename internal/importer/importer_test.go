package importer

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/scope"
	"github.com/sloppy/nmaptracker/internal/testutil"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := testutil.TempDir(t)
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database
}

func mustMatcher(t *testing.T, defs []string) *scope.Matcher {
	t.Helper()
	m, err := scope.NewMatcher(defs)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	return m
}

func TestImportAdditiveAndEnrichment(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	project, err := database.CreateProject("proj")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	matcher := mustMatcher(t, []string{"0.0.0.0/0"})
	now1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	obs1 := Observations{
		Hosts: []HostObservation{
			{
				IPAddress: "10.0.0.1",
				Hostname:  "alpha",
				Ports: []PortObservation{
					{PortNumber: 80, Protocol: "tcp", State: "open", Service: "http"},
					{PortNumber: 443, Protocol: "tcp", State: "open", Service: ""},
				},
			},
		},
	}
	if _, err := ImportObservations(database, matcher, project.ID, "scan1.xml", obs1, now1); err != nil {
		t.Fatalf("import1: %v", err)
	}

	// Mark port 443 as flagged to ensure work_status is preserved on enrichment.
	host, _, err := database.GetHostByIP(project.ID, "10.0.0.1")
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 443,
		Protocol:   "tcp",
		State:      "open",
		Service:    "https",
		WorkStatus: "flagged",
		LastSeen:   now1,
	}); err != nil {
		t.Fatalf("flag port: %v", err)
	}

	now2 := now1.Add(time.Hour)
	obs2 := Observations{
		Hosts: []HostObservation{
			{
				IPAddress: "10.0.0.1",
				Ports: []PortObservation{
					{PortNumber: 443, Protocol: "tcp", State: "open", Service: "https", Version: "1.2.3", Product: "nginx", ExtraInfo: "stable"},
					{PortNumber: 22, Protocol: "tcp", State: "open", Service: "ssh"},
				},
			},
		},
	}
	if _, err := ImportObservations(database, matcher, project.ID, "scan2.xml", obs2, now2); err != nil {
		t.Fatalf("import2: %v", err)
	}

	ports, err := database.ListPorts(host.ID)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports after additive import, got %d", len(ports))
	}

	var p80, p443, p22 db.Port
	for _, p := range ports {
		switch {
		case p.PortNumber == 80 && p.Protocol == "tcp":
			p80 = p
		case p.PortNumber == 443 && p.Protocol == "tcp":
			p443 = p
		case p.PortNumber == 22 && p.Protocol == "tcp":
			p22 = p
		}
	}
	if p80.ID == 0 {
		t.Fatalf("expected port 80 to remain after second import")
	}
	if p22.ID == 0 {
		t.Fatalf("expected new port 22 to be added")
	}
	if p443.WorkStatus != "flagged" {
		t.Fatalf("expected work_status to be preserved, got %s", p443.WorkStatus)
	}
	if p443.Service != "https" || p443.Version != "1.2.3" || p443.Product != "nginx" || p443.ExtraInfo != "stable" {
		t.Fatalf("expected enrichment on port 443, got %+v", p443)
	}
	if !p443.LastSeen.Equal(now2) {
		t.Fatalf("expected last_seen updated to now2, got %v", p443.LastSeen)
	}
}

func TestImportProtocolAware(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	project, _ := database.CreateProject("proto")
	matcher := mustMatcher(t, []string{"0.0.0.0/0"})
	now := time.Now().UTC()
	obs := Observations{
		Hosts: []HostObservation{
			{
				IPAddress: "192.0.2.10",
				Ports: []PortObservation{
					{PortNumber: 53, Protocol: "tcp", State: "open", Service: "domain"},
					{PortNumber: 53, Protocol: "udp", State: "open", Service: "domain"},
				},
			},
		},
	}
	if _, err := ImportObservations(database, matcher, project.ID, "proto.xml", obs, now); err != nil {
		t.Fatalf("import: %v", err)
	}
	host, _, _ := database.GetHostByIP(project.ID, "192.0.2.10")
	ports, err := database.ListPorts(host.ID)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 protocol-distinct ports, got %d", len(ports))
	}
}

func TestImportScopeChangeUpdatesHost(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()
	project, _ := database.CreateProject("scope")

	matcherInclude := mustMatcher(t, []string{"10.0.0.0/24"})
	obs := Observations{
		Hosts: []HostObservation{{IPAddress: "10.0.0.5", Ports: []PortObservation{{PortNumber: 80, Protocol: "tcp", State: "open", Service: "http"}}}},
	}
	if _, err := ImportObservations(database, matcherInclude, project.ID, "scan1.xml", obs, time.Now().UTC()); err != nil {
		t.Fatalf("import include: %v", err)
	}
	host, _, _ := database.GetHostByIP(project.ID, "10.0.0.5")
	if !host.InScope {
		t.Fatalf("expected host to be in scope initially")
	}

	// New logic: simpler, strict allow-list logic.
	// If we want to simulate exclude, we just don't include it in the allowed list,
	// assuming we have a way to define allowed list strictly.
	// In this test, let's just use a different specific subnet that doesn't include 10.0.0.5.

	matcherExclude := mustMatcher(t, []string{"192.168.0.0/16"}) // 10.0.0.5 is NOT in this

	if _, err := ImportObservations(database, matcherExclude, project.ID, "scan2.xml", obs, time.Now().UTC()); err != nil {
		t.Fatalf("import exclude: %v", err)
	}
	host, _, _ = database.GetHostByIP(project.ID, "10.0.0.5")
	if host.InScope {
		t.Fatalf("expected host to be marked out-of-scope after scope change")
	}
}

func TestImportTransactionRollbackOnError(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	project, err := database.CreateProject("rollback")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	matcher := mustMatcher(t, []string{"0.0.0.0/0"})
	obs := Observations{
		Hosts: []HostObservation{
			{IPAddress: "10.10.10.10", Ports: []PortObservation{{PortNumber: 80, Protocol: "tcp", State: "open"}}},
			{IPAddress: "bad-ip", Ports: []PortObservation{{PortNumber: 443, Protocol: "tcp", State: "open"}}},
		},
	}

	if _, err := ImportObservations(database, matcher, project.ID, "bad.xml", obs, time.Now().UTC()); err == nil {
		t.Fatalf("expected import error")
	}

	imports, err := database.ListScanImports(project.ID)
	if err != nil {
		t.Fatalf("list scan imports: %v", err)
	}
	if len(imports) != 0 {
		t.Fatalf("expected no scan imports after rollback")
	}

	hosts, err := database.ListHosts(project.ID)
	if err != nil {
		t.Fatalf("list hosts: %v", err)
	}
	if len(hosts) != 0 {
		t.Fatalf("expected no hosts after rollback")
	}
}
