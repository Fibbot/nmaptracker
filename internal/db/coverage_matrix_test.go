package db

import (
	"errors"
	"strconv"
	"testing"
)

func TestCoverageMatrixScopeSegments(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("coverage-scope")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	scopeCIDR, err := db.AddScopeDefinition(project.ID, "10.0.0.0/24", "cidr")
	if err != nil {
		t.Fatalf("add scope cidr: %v", err)
	}
	scopeIP, err := db.AddScopeDefinition(project.ID, "10.0.1.10", "ip")
	if err != nil {
		t.Fatalf("add scope ip: %v", err)
	}

	h1, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.5", InScope: true})
	h2, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.1.10", InScope: true})
	h3, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.2.3", InScope: true})
	_, _ = db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.99", InScope: false})

	if err := insertCoverageImport(db, project.ID, "ping.xml", IntentPingSweep, []HostObservation{
		{ProjectID: project.ID, IPAddress: h1.IPAddress, InScope: true, HostState: "up"},
		{ProjectID: project.ID, IPAddress: h2.IPAddress, InScope: true, HostState: "up"},
	}); err != nil {
		t.Fatalf("insert ping import: %v", err)
	}
	if err := insertCoverageImport(db, project.ID, "top1k.xml", IntentTop1KTCP, []HostObservation{
		{ProjectID: project.ID, IPAddress: h1.IPAddress, InScope: true, HostState: "up"},
	}); err != nil {
		t.Fatalf("insert top1k import: %v", err)
	}

	matrix, err := db.GetCoverageMatrix(project.ID, CoverageMatrixOptions{IncludeMissingPreview: true, MissingPreviewSize: 5})
	if err != nil {
		t.Fatalf("get coverage matrix: %v", err)
	}

	if matrix.SegmentMode != "scope_rules" {
		t.Fatalf("expected scope_rules mode, got %q", matrix.SegmentMode)
	}

	rows := matrixSegmentsByKey(matrix.Segments)
	cidrKey := "scope:" + intToString(scopeCIDR.ID)
	ipKey := "scope:" + intToString(scopeIP.ID)

	if rows[cidrKey].HostTotal != 1 || rows[ipKey].HostTotal != 1 || rows["scope:unmapped"].HostTotal != 1 {
		t.Fatalf("unexpected host totals: cidr=%d ip=%d unmapped=%d",
			rows[cidrKey].HostTotal, rows[ipKey].HostTotal, rows["scope:unmapped"].HostTotal)
	}

	pingCIDR := rows[cidrKey].Cells[IntentPingSweep]
	if pingCIDR.CoveredCount != 1 || pingCIDR.MissingCount != 0 || pingCIDR.CoveragePercent != 100 {
		t.Fatalf("unexpected ping coverage for cidr segment: %+v", pingCIDR)
	}

	top1kIP := rows[ipKey].Cells[IntentTop1KTCP]
	if top1kIP.CoveredCount != 0 || top1kIP.MissingCount != 1 || top1kIP.CoveragePercent != 0 {
		t.Fatalf("unexpected top_1k_tcp coverage for ip segment: %+v", top1kIP)
	}
	if len(top1kIP.MissingHosts) != 1 || top1kIP.MissingHosts[0].IPAddress != h2.IPAddress {
		t.Fatalf("unexpected missing preview: %+v", top1kIP.MissingHosts)
	}

	unmappedPing := rows["scope:unmapped"].Cells[IntentPingSweep]
	if unmappedPing.MissingCount != 1 || len(unmappedPing.MissingHosts) != 1 || unmappedPing.MissingHosts[0].IPAddress != h3.IPAddress {
		t.Fatalf("unexpected unmapped ping cell: %+v", unmappedPing)
	}
}

func TestCoverageMatrixFallbackAndMissingPagination(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("coverage-fallback")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	h1, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.1.5", InScope: true})
	h2, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.1.20", InScope: true})
	h3, _ := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.1.2.5", InScope: true})

	if err := insertCoverageImport(db, project.ID, "udp.xml", IntentTopUDP, []HostObservation{
		{ProjectID: project.ID, IPAddress: h1.IPAddress, InScope: true, HostState: "up"},
	}); err != nil {
		t.Fatalf("insert udp import: %v", err)
	}

	matrix, err := db.GetCoverageMatrix(project.ID, CoverageMatrixOptions{IncludeMissingPreview: false, MissingPreviewSize: 5})
	if err != nil {
		t.Fatalf("get coverage matrix: %v", err)
	}
	if matrix.SegmentMode != "fallback_24" {
		t.Fatalf("expected fallback_24 mode, got %q", matrix.SegmentMode)
	}

	rows := matrixSegmentsByKey(matrix.Segments)
	segmentA, ok := rows["fallback:10.1.1.0/24"]
	if !ok {
		t.Fatalf("expected fallback:10.1.1.0/24 segment")
	}
	if segmentA.HostTotal != 2 {
		t.Fatalf("expected 2 hosts in first fallback segment, got %d", segmentA.HostTotal)
	}
	if len(segmentA.Cells[IntentTopUDP].MissingHosts) != 0 {
		t.Fatalf("expected no preview hosts when include preview disabled")
	}

	segmentB, ok := rows["fallback:10.1.2.0/24"]
	if !ok {
		t.Fatalf("expected fallback:10.1.2.0/24 segment")
	}
	if segmentB.Cells[IntentTopUDP].CoveredCount != 0 || segmentB.Cells[IntentTopUDP].MissingCount != 1 {
		t.Fatalf("unexpected segmentB top_udp cell: %+v", segmentB.Cells[IntentTopUDP])
	}

	missing, total, err := db.ListCoverageMatrixMissingHosts(project.ID, CoverageMatrixMissingOptions{
		SegmentKey: "fallback:10.1.1.0/24",
		Intent:     IntentTopUDP,
		Page:       1,
		PageSize:   1,
	})
	if err != nil {
		t.Fatalf("list missing hosts: %v", err)
	}
	if total != 1 || len(missing) != 1 || missing[0].IPAddress != h2.IPAddress {
		t.Fatalf("unexpected missing pagination result: total=%d items=%+v", total, missing)
	}

	if _, _, err := db.ListCoverageMatrixMissingHosts(project.ID, CoverageMatrixMissingOptions{
		SegmentKey: "scope:not-real",
		Intent:     IntentTopUDP,
		Page:       1,
		PageSize:   10,
	}); !errors.Is(err, ErrCoverageSegmentNotFound) {
		t.Fatalf("expected ErrCoverageSegmentNotFound, got %v", err)
	}

	if _, _, err := db.ListCoverageMatrixMissingHosts(project.ID, CoverageMatrixMissingOptions{
		SegmentKey: "fallback:10.1.1.0/24",
		Intent:     "unknown",
		Page:       1,
		PageSize:   10,
	}); err == nil {
		t.Fatalf("expected invalid intent error")
	}

	_ = h3
}

func insertCoverageImport(db *DB, projectID int64, filename, intent string, hosts []HostObservation) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	imp, err := tx.InsertScanImport(ScanImport{ProjectID: projectID, Filename: filename})
	if err != nil {
		return err
	}
	if _, err := tx.InsertScanImportIntent(ScanImportIntent{
		ScanImportID: imp.ID,
		Intent:       intent,
		Source:       IntentSourceManual,
		Confidence:   1.0,
	}); err != nil {
		return err
	}

	for _, h := range hosts {
		h.ScanImportID = imp.ID
		h.ProjectID = projectID
		if _, err := tx.InsertHostObservation(h); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func matrixSegmentsByKey(rows []CoverageMatrixSegment) map[string]CoverageMatrixSegment {
	out := make(map[string]CoverageMatrixSegment, len(rows))
	for _, row := range rows {
		out[row.SegmentKey] = row
	}
	return out
}

func intToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
