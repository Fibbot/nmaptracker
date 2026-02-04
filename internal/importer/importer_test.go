package importer

import (
	"path/filepath"
	"strings"
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

	var hostObsCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM host_observation WHERE project_id = ?`, project.ID).Scan(&hostObsCount); err != nil {
		t.Fatalf("count host observations: %v", err)
	}
	if hostObsCount != 0 {
		t.Fatalf("expected no host observations after rollback, got %d", hostObsCount)
	}

	var portObsCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM port_observation WHERE project_id = ?`, project.ID).Scan(&portObsCount); err != nil {
		t.Fatalf("count port observations: %v", err)
	}
	if portObsCount != 0 {
		t.Fatalf("expected no port observations after rollback, got %d", portObsCount)
	}

	var intentCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM scan_import_intent`).Scan(&intentCount); err != nil {
		t.Fatalf("count intents: %v", err)
	}
	if intentCount != 0 {
		t.Fatalf("expected no intents after rollback, got %d", intentCount)
	}
}

func TestImportXMLWithOptionsPersistsAutoAndManualIntents(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	project, err := database.CreateProject("intents")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	matcher := mustMatcher(t, []string{"0.0.0.0/0"})
	xmlPayload := `<?xml version="1.0"?>
<nmaprun args="nmap -sn -sU --top-ports 1000 -p- --script vuln 192.0.2.10">
  <host>
    <status state="up"/>
    <address addr="192.0.2.10" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	stats, err := ImportXMLWithOptions(
		database,
		matcher,
		project.ID,
		"scan.xml",
		strings.NewReader(xmlPayload),
		ImportOptions{ManualIntents: []string{db.IntentTop1KTCP}},
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("import xml with options: %v", err)
	}

	imports, err := database.ListScanImportsWithIntents(project.ID)
	if err != nil {
		t.Fatalf("list imports with intents: %v", err)
	}
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	if imports[0].ID != stats.ScanImport.ID {
		t.Fatalf("unexpected import id %d", imports[0].ID)
	}

	sourcesByIntent := map[string]string{}
	for _, intent := range imports[0].Intents {
		sourcesByIntent[intent.Intent] = intent.Source
	}

	if sourcesByIntent[db.IntentTop1KTCP] != db.IntentSourceManual {
		t.Fatalf("expected %s to remain manual, got %q", db.IntentTop1KTCP, sourcesByIntent[db.IntentTop1KTCP])
	}
	if sourcesByIntent[db.IntentPingSweep] != db.IntentSourceAuto {
		t.Fatalf("expected ping_sweep auto source")
	}
	if sourcesByIntent[db.IntentAllTCP] != db.IntentSourceAuto {
		t.Fatalf("expected all_tcp auto source")
	}
	if sourcesByIntent[db.IntentTopUDP] != db.IntentSourceAuto {
		t.Fatalf("expected top_udp auto source")
	}
	if sourcesByIntent[db.IntentVulnNSE] != db.IntentSourceAuto {
		t.Fatalf("expected vuln_nse auto source")
	}
}

func TestImportXMLPersistsHostAndPortObservations(t *testing.T) {
	database := newTestDB(t)
	defer database.Close()

	project, err := database.CreateProject("observations")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	matcher := mustMatcher(t, []string{"0.0.0.0/0"})
	xmlPayload := `<?xml version="1.0"?>
<nmaprun args="nmap --top-ports 1000 198.51.100.5">
  <host>
    <status state="up"/>
    <address addr="198.51.100.5" addrtype="ipv4"/>
    <hostnames><hostname name="scan-host"/></hostnames>
    <ports>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https" product="nginx" version="1.25"/>
      </port>
    </ports>
  </host>
</nmaprun>`

	stats, err := ImportXML(database, matcher, project.ID, "obs.xml", strings.NewReader(xmlPayload), time.Now().UTC())
	if err != nil {
		t.Fatalf("import xml: %v", err)
	}

	hostObs, err := database.ListHostObservationsByImport(project.ID, stats.ScanImport.ID)
	if err != nil {
		t.Fatalf("list host observations: %v", err)
	}
	if len(hostObs) != 1 {
		t.Fatalf("expected 1 host observation, got %d", len(hostObs))
	}
	if hostObs[0].IPAddress != "198.51.100.5" || hostObs[0].HostState != "up" {
		t.Fatalf("unexpected host observation: %#v", hostObs[0])
	}

	portObs, err := database.ListPortObservationsByImport(project.ID, stats.ScanImport.ID)
	if err != nil {
		t.Fatalf("list port observations: %v", err)
	}
	if len(portObs) != 1 {
		t.Fatalf("expected 1 port observation, got %d", len(portObs))
	}
	if portObs[0].PortNumber != 443 || portObs[0].Protocol != "tcp" || portObs[0].State != "open" {
		t.Fatalf("unexpected port observation: %#v", portObs[0])
	}
}
