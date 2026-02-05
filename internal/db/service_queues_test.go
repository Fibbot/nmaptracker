package db

import (
	"testing"
	"time"
)

func TestServiceCampaignQueueCampaignMatching(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("service-campaign-matching")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	seed := []struct {
		ip       string
		inScope  bool
		port     int
		protocol string
		state    string
		service  string
	}{
		{ip: "10.0.0.10", inScope: true, port: 445, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.11", inScope: true, port: 139, protocol: "tcp", state: "open", service: "netbios-ssn"},
		{ip: "10.0.0.12", inScope: true, port: 636, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.13", inScope: true, port: 9000, protocol: "tcp", state: "open", service: "ldaps"},
		{ip: "10.0.0.14", inScope: true, port: 3389, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.15", inScope: true, port: 3390, protocol: "tcp", state: "open", service: "ms-wbt-server"},
		{ip: "10.0.0.16", inScope: true, port: 8080, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.17", inScope: true, port: 9001, protocol: "tcp", state: "open", service: "http-proxy"},
		{ip: "10.0.0.20", inScope: true, port: 9443, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.21", inScope: true, port: 81, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.22", inScope: true, port: 8081, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.23", inScope: true, port: 8888, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.24", inScope: true, port: 22, protocol: "tcp", state: "open", service: ""},
		{ip: "10.0.0.25", inScope: true, port: 2022, protocol: "tcp", state: "open", service: "openssh"},
		{ip: "10.0.0.18", inScope: false, port: 445, protocol: "tcp", state: "open", service: "microsoft-ds"},
		{ip: "10.0.0.19", inScope: true, port: 80, protocol: "tcp", state: "closed", service: "http"},
	}

	for _, row := range seed {
		host, err := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: row.ip, InScope: row.inScope})
		if err != nil {
			t.Fatalf("upsert host %s: %v", row.ip, err)
		}
		if _, err := db.UpsertPort(Port{
			HostID:     host.ID,
			PortNumber: row.port,
			Protocol:   row.protocol,
			State:      row.state,
			Service:    row.service,
			WorkStatus: "scanned",
		}); err != nil {
			t.Fatalf("upsert port for host %s: %v", row.ip, err)
		}
	}

	cases := []struct {
		campaigns []string
		wantIPs   []string
	}{
		{campaigns: []string{ServiceCampaignSMB}, wantIPs: []string{"10.0.0.10", "10.0.0.11"}},
		{campaigns: []string{ServiceCampaignLDAP}, wantIPs: []string{"10.0.0.12", "10.0.0.13"}},
		{campaigns: []string{ServiceCampaignRDP}, wantIPs: []string{"10.0.0.14", "10.0.0.15"}},
		{campaigns: []string{ServiceCampaignHTTP}, wantIPs: []string{"10.0.0.16", "10.0.0.17", "10.0.0.20"}},
		{campaigns: []string{ServiceCampaignSSH}, wantIPs: []string{"10.0.0.24", "10.0.0.25"}},
		{campaigns: []string{"ssh,http"}, wantIPs: []string{"10.0.0.16", "10.0.0.17", "10.0.0.20", "10.0.0.24", "10.0.0.25"}},
	}

	for _, tc := range cases {
		items, total, sourceIDs, err := db.ListServiceCampaignQueue(project.ID, tc.campaigns, 100, 0)
		if err != nil {
			t.Fatalf("list service queue %+v: %v", tc.campaigns, err)
		}
		if total != len(tc.wantIPs) {
			t.Fatalf("%+v: expected total %d, got %d", tc.campaigns, len(tc.wantIPs), total)
		}
		if len(items) != len(tc.wantIPs) {
			t.Fatalf("%+v: expected %d items, got %d", tc.campaigns, len(tc.wantIPs), len(items))
		}
		if len(sourceIDs) != 0 {
			t.Fatalf("%+v: expected no source_import_ids without observations, got %+v", tc.campaigns, sourceIDs)
		}
		for i, item := range items {
			if item.IPAddress != tc.wantIPs[i] {
				t.Fatalf("%+v: expected item[%d].ip=%s, got %s", tc.campaigns, i, tc.wantIPs[i], item.IPAddress)
			}
		}
	}

	if _, _, _, err := db.ListServiceCampaignQueue(project.ID, []string{"not_real"}, 10, 0); err == nil {
		t.Fatalf("expected invalid campaign error")
	}
}

func TestServiceCampaignQueueGroupingStatusSummaryPaginationAndAudit(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("service-campaign-grouping")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	t1 := time.Date(2026, time.February, 1, 1, 0, 0, 0, time.UTC)
	t2 := t1.Add(2 * time.Hour)
	t3 := t1.Add(4 * time.Hour)
	t4 := t1.Add(6 * time.Hour)

	hostA, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.0.1", Hostname: "alpha", InScope: true})
	hostB, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.0.2", Hostname: "beta", InScope: true})
	hostC, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.0.3", Hostname: "charlie", InScope: true})

	if _, err := db.UpsertPort(Port{
		HostID:     hostA.ID,
		PortNumber: 445,
		Protocol:   "tcp",
		State:      "open",
		Service:    "microsoft-ds",
		WorkStatus: "scanned",
		LastSeen:   t1,
	}); err != nil {
		t.Fatalf("upsert hostA smb port 445: %v", err)
	}
	if _, err := db.UpsertPort(Port{
		HostID:     hostA.ID,
		PortNumber: 139,
		Protocol:   "tcp",
		State:      "open|filtered",
		Service:    "netbios-ssn",
		WorkStatus: "flagged",
		LastSeen:   t2,
	}); err != nil {
		t.Fatalf("upsert hostA smb port 139: %v", err)
	}
	if _, err := db.UpsertPort(Port{
		HostID:     hostB.ID,
		PortNumber: 9000,
		Protocol:   "tcp",
		State:      "open",
		Service:    "smb",
		WorkStatus: "done",
		LastSeen:   t3,
	}); err != nil {
		t.Fatalf("upsert hostB smb service port: %v", err)
	}
	if _, err := db.UpsertPort(Port{
		HostID:     hostC.ID,
		PortNumber: 445,
		Protocol:   "tcp",
		State:      "open",
		Service:    "microsoft-ds",
		WorkStatus: "flagged",
		LastSeen:   t4,
	}); err != nil {
		t.Fatalf("upsert hostC smb port: %v", err)
	}

	importA1, err := insertServiceQueueImportObservations(db, project.ID, []string{hostA.IPAddress})
	if err != nil {
		t.Fatalf("insert importA1: %v", err)
	}
	importA2, err := insertServiceQueueImportObservations(db, project.ID, []string{hostA.IPAddress})
	if err != nil {
		t.Fatalf("insert importA2: %v", err)
	}
	importC, err := insertServiceQueueImportObservations(db, project.ID, []string{hostC.IPAddress})
	if err != nil {
		t.Fatalf("insert importC: %v", err)
	}

	firstPage, total, sourceIDs, err := db.ListServiceCampaignQueue(project.ID, []string{ServiceCampaignSMB}, 2, 0)
	if err != nil {
		t.Fatalf("list first page queue: %v", err)
	}
	if total != 3 || len(firstPage) != 2 {
		t.Fatalf("unexpected first page shape total=%d items=%d", total, len(firstPage))
	}
	if firstPage[0].IPAddress != hostA.IPAddress || firstPage[1].IPAddress != hostB.IPAddress {
		t.Fatalf("unexpected first page host ordering: %+v", firstPage)
	}
	if len(firstPage[0].MatchingPorts) != 2 {
		t.Fatalf("expected 2 matching ports for hostA, got %d", len(firstPage[0].MatchingPorts))
	}
	if firstPage[0].StatusSummary.Scanned != 1 || firstPage[0].StatusSummary.Flagged != 1 {
		t.Fatalf("unexpected hostA status summary: %+v", firstPage[0].StatusSummary)
	}
	if !firstPage[0].LatestSeen.Equal(t2) {
		t.Fatalf("expected hostA latest_seen=%v, got %v", t2, firstPage[0].LatestSeen)
	}
	if firstPage[1].StatusSummary.Done != 1 {
		t.Fatalf("expected hostB done summary count, got %+v", firstPage[1].StatusSummary)
	}
	if !firstPage[1].LatestSeen.Equal(t3) {
		t.Fatalf("expected hostB latest_seen=%v, got %v", t3, firstPage[1].LatestSeen)
	}
	if len(sourceIDs) != 2 || sourceIDs[0] != importA1 || sourceIDs[1] != importA2 {
		t.Fatalf("unexpected first page source_import_ids: %+v", sourceIDs)
	}

	secondPage, _, sourceIDsPage2, err := db.ListServiceCampaignQueue(project.ID, []string{ServiceCampaignSMB}, 2, 2)
	if err != nil {
		t.Fatalf("list second page queue: %v", err)
	}
	if len(secondPage) != 1 || secondPage[0].IPAddress != hostC.IPAddress {
		t.Fatalf("unexpected second page items: %+v", secondPage)
	}
	if secondPage[0].StatusSummary.Flagged != 1 {
		t.Fatalf("expected hostC flagged status summary, got %+v", secondPage[0].StatusSummary)
	}
	if len(sourceIDsPage2) != 1 || sourceIDsPage2[0] != importC {
		t.Fatalf("unexpected second page source_import_ids: %+v", sourceIDsPage2)
	}
}

func insertServiceQueueImportObservations(db *DB, projectID int64, ips []string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(ScanImport{
		ProjectID: projectID,
		Filename:  "service-queue.xml",
	})
	if err != nil {
		return 0, err
	}
	for _, ip := range ips {
		if _, err := tx.InsertHostObservation(HostObservation{
			ScanImportID: record.ID,
			ProjectID:    projectID,
			IPAddress:    ip,
			InScope:      true,
			HostState:    "up",
		}); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return record.ID, nil
}
