package db

import (
	"testing"
)

func TestMilestoneQueuesPrecedence(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("milestone-precedence")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	hPing, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.50.0.1", InScope: true})
	hTop1k, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.50.0.2", InScope: true})
	hAll, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.50.0.3", InScope: true})
	hNone, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.50.0.4", InScope: true})

	if err := insertGapImportWithObservations(db, project.ID, IntentPingSweep, []string{hPing.IPAddress}); err != nil {
		t.Fatalf("insert ping import: %v", err)
	}
	if err := insertGapImportWithObservations(db, project.ID, IntentTop1KTCP, []string{hTop1k.IPAddress}); err != nil {
		t.Fatalf("insert top1k import: %v", err)
	}
	if err := insertGapImportWithObservations(db, project.ID, IntentAllTCP, []string{hAll.IPAddress}); err != nil {
		t.Fatalf("insert all import: %v", err)
	}

	queues, err := db.GetMilestoneQueues(project.ID, GapOptions{PreviewSize: 20, IncludeLists: true})
	if err != nil {
		t.Fatalf("get milestone queues: %v", err)
	}

	if queues.Summary.NeedsPingSweep != 1 {
		t.Fatalf("expected needs_ping_sweep=1, got %d", queues.Summary.NeedsPingSweep)
	}
	if queues.Summary.NeedsTop1KTCP != 2 {
		t.Fatalf("expected needs_top_1k_tcp=2, got %d", queues.Summary.NeedsTop1KTCP)
	}
	if queues.Summary.NeedsAllTCP != 3 {
		t.Fatalf("expected needs_all_tcp=3, got %d", queues.Summary.NeedsAllTCP)
	}

	if queues.Lists == nil {
		t.Fatalf("expected milestone lists")
	}

	if len(queues.Lists.NeedsPingSweep) != 1 || queues.Lists.NeedsPingSweep[0].IPAddress != hNone.IPAddress {
		t.Fatalf("unexpected needs_ping_sweep list: %+v", queues.Lists.NeedsPingSweep)
	}

	needTopSet := make(map[string]struct{})
	for _, host := range queues.Lists.NeedsTop1KTCP {
		needTopSet[host.IPAddress] = struct{}{}
	}
	if _, ok := needTopSet[hPing.IPAddress]; !ok {
		t.Fatalf("expected ping-only host to need top_1k_tcp")
	}
	if _, ok := needTopSet[hNone.IPAddress]; !ok {
		t.Fatalf("expected unobserved host to need top_1k_tcp")
	}

	needAllSet := make(map[string]struct{})
	for _, host := range queues.Lists.NeedsAllTCP {
		needAllSet[host.IPAddress] = struct{}{}
	}
	if _, ok := needAllSet[hPing.IPAddress]; !ok {
		t.Fatalf("expected ping-only host to need all_tcp")
	}
	if _, ok := needAllSet[hTop1k.IPAddress]; !ok {
		t.Fatalf("expected top1k-only host to need all_tcp")
	}
	if _, ok := needAllSet[hNone.IPAddress]; !ok {
		t.Fatalf("expected unobserved host to need all_tcp")
	}
}

func TestGapDashboardSummaryAndPreviews(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("gap-dashboard")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	h1, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.60.0.1", InScope: true})
	h2, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.60.0.2", InScope: true})
	h3, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.60.0.3", InScope: false})

	if _, err := db.UpsertPort(Port{HostID: h1.ID, PortNumber: 80, Protocol: "tcp", State: "open", WorkStatus: "scanned", Service: "http"}); err != nil {
		t.Fatalf("insert h1 scanned port: %v", err)
	}
	if _, err := db.UpsertPort(Port{HostID: h1.ID, PortNumber: 443, Protocol: "tcp", State: "open", WorkStatus: "done", Service: "https"}); err != nil {
		t.Fatalf("insert h1 done port: %v", err)
	}
	if _, err := db.UpsertPort(Port{HostID: h2.ID, PortNumber: 53, Protocol: "udp", State: "open", WorkStatus: "flagged", Service: "domain"}); err != nil {
		t.Fatalf("insert h2 flagged port: %v", err)
	}
	if _, err := db.UpsertPort(Port{HostID: h3.ID, PortNumber: 22, Protocol: "tcp", State: "open", WorkStatus: "scanned", Service: "ssh"}); err != nil {
		t.Fatalf("insert h3 scanned port: %v", err)
	}

	if err := insertGapImportWithObservations(db, project.ID, IntentAllTCP, []string{h1.IPAddress}); err != nil {
		t.Fatalf("insert all tcp import: %v", err)
	}

	gaps, err := db.GetGapDashboard(project.ID, GapOptions{PreviewSize: 1, IncludeLists: true})
	if err != nil {
		t.Fatalf("get gap dashboard: %v", err)
	}

	if gaps.Summary.InScopeNeverScanned != 1 {
		t.Fatalf("expected in_scope_never_scanned=1, got %d", gaps.Summary.InScopeNeverScanned)
	}
	if gaps.Summary.OpenPortsScannedOrFlagged != 2 {
		t.Fatalf("expected open_ports_scanned_or_flagged=2, got %d", gaps.Summary.OpenPortsScannedOrFlagged)
	}
	if gaps.Summary.NeedsPingSweep != 1 || gaps.Summary.NeedsTop1KTCP != 1 || gaps.Summary.NeedsAllTCP != 1 {
		t.Fatalf("unexpected milestone summary: %+v", gaps.Summary)
	}

	if gaps.Lists == nil {
		t.Fatalf("expected lists in gap response")
	}
	if len(gaps.Lists.InScopeNeverScanned) != 1 || gaps.Lists.InScopeNeverScanned[0].IPAddress != h2.IPAddress {
		t.Fatalf("unexpected never-scanned preview: %+v", gaps.Lists.InScopeNeverScanned)
	}
	if len(gaps.Lists.OpenPortsScannedOrFlagged) != 1 {
		t.Fatalf("expected open-port preview limited to 1 row")
	}
	if len(gaps.Lists.OpenPortsScannedOrFlaggedGrouped) != 1 {
		t.Fatalf("expected grouped preview limited to 1 host")
	}

	withoutLists, err := db.GetGapDashboard(project.ID, GapOptions{PreviewSize: 10, IncludeLists: false})
	if err != nil {
		t.Fatalf("get gap dashboard without lists: %v", err)
	}
	if withoutLists.Lists != nil {
		t.Fatalf("expected no lists when include_lists=false")
	}
}

func insertGapImportWithObservations(db *DB, projectID int64, intent string, ips []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(ScanImport{ProjectID: projectID, Filename: intent + ".xml"})
	if err != nil {
		return err
	}

	if _, err := tx.InsertScanImportIntent(ScanImportIntent{
		ScanImportID: record.ID,
		Intent:       intent,
		Source:       IntentSourceManual,
		Confidence:   1.0,
	}); err != nil {
		return err
	}

	for _, ip := range ips {
		if _, err := tx.InsertHostObservation(HostObservation{
			ScanImportID: record.ID,
			ProjectID:    projectID,
			IPAddress:    ip,
			InScope:      true,
			HostState:    "up",
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}
