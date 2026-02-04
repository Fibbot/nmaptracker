package db

import (
	"errors"
	"testing"
)

func TestGetImportDeltaClassifiesHostAndExposureChanges(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, err := db.CreateProject("delta")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	baseID, err := insertDeltaImportFixture(db, project.ID, "base.xml",
		[]HostObservation{
			{ProjectID: project.ID, IPAddress: "10.0.0.1", Hostname: "alpha", InScope: true, HostState: "up"},
			{ProjectID: project.ID, IPAddress: "10.0.0.2", Hostname: "bravo", InScope: true, HostState: "up"},
		},
		[]PortObservation{
			{ProjectID: project.ID, IPAddress: "10.0.0.1", PortNumber: 80, Protocol: "tcp", State: "open", Service: "http", Product: "nginx", Version: "1.20"},
			{ProjectID: project.ID, IPAddress: "10.0.0.2", PortNumber: 22, Protocol: "tcp", State: "open", Service: "ssh", Product: "openssh", Version: "8.0"},
			{ProjectID: project.ID, IPAddress: "10.0.0.1", PortNumber: 53, Protocol: "udp", State: "closed", Service: "domain"},
		},
	)
	if err != nil {
		t.Fatalf("insert base import fixture: %v", err)
	}

	targetID, err := insertDeltaImportFixture(db, project.ID, "target.xml",
		[]HostObservation{
			{ProjectID: project.ID, IPAddress: "10.0.0.1", Hostname: "alpha", InScope: true, HostState: "up"},
			{ProjectID: project.ID, IPAddress: "10.0.0.3", Hostname: "charlie", InScope: true, HostState: "up"},
		},
		[]PortObservation{
			{ProjectID: project.ID, IPAddress: "10.0.0.1", PortNumber: 80, Protocol: "tcp", State: "open", Service: "http", Product: "nginx", Version: "1.24"},
			{ProjectID: project.ID, IPAddress: "10.0.0.3", PortNumber: 443, Protocol: "tcp", State: "open|filtered", Service: "https", Product: "nginx", Version: "1.24"},
			{ProjectID: project.ID, IPAddress: "10.0.0.3", PortNumber: 25, Protocol: "tcp", State: "filtered", Service: "smtp"},
		},
	)
	if err != nil {
		t.Fatalf("insert target import fixture: %v", err)
	}

	delta, err := db.GetImportDelta(project.ID, baseID, targetID, DeltaOptions{PreviewSize: 100, IncludeLists: true})
	if err != nil {
		t.Fatalf("get import delta: %v", err)
	}

	if delta.Summary.NetNewHosts != 1 || delta.Summary.DisappearedHosts != 1 {
		t.Fatalf("unexpected host summary: %+v", delta.Summary)
	}
	if delta.Summary.NetNewOpenExposures != 1 || delta.Summary.DisappearedOpenExposures != 1 {
		t.Fatalf("unexpected exposure summary: %+v", delta.Summary)
	}
	if delta.Summary.ChangedServiceFingerprints != 1 {
		t.Fatalf("expected one changed fingerprint, got %d", delta.Summary.ChangedServiceFingerprints)
	}

	if delta.Lists == nil {
		t.Fatalf("expected lists in delta response")
	}
	if len(delta.Lists.NetNewHosts) != 1 || delta.Lists.NetNewHosts[0].IPAddress != "10.0.0.3" {
		t.Fatalf("unexpected net_new_hosts list: %+v", delta.Lists.NetNewHosts)
	}
	if len(delta.Lists.DisappearedHosts) != 1 || delta.Lists.DisappearedHosts[0].IPAddress != "10.0.0.2" {
		t.Fatalf("unexpected disappeared_hosts list: %+v", delta.Lists.DisappearedHosts)
	}
	if len(delta.Lists.NetNewOpenExposures) != 1 || delta.Lists.NetNewOpenExposures[0].PortNumber != 443 {
		t.Fatalf("unexpected net_new_open_exposures list: %+v", delta.Lists.NetNewOpenExposures)
	}
	if len(delta.Lists.DisappearedOpenExposures) != 1 || delta.Lists.DisappearedOpenExposures[0].PortNumber != 22 {
		t.Fatalf("unexpected disappeared_open_exposures list: %+v", delta.Lists.DisappearedOpenExposures)
	}
	if len(delta.Lists.ChangedServiceFingerprints) != 1 {
		t.Fatalf("unexpected changed_service_fingerprints list: %+v", delta.Lists.ChangedServiceFingerprints)
	}
	change := delta.Lists.ChangedServiceFingerprints[0]
	if change.IPAddress != "10.0.0.1" || change.PortNumber != 80 {
		t.Fatalf("unexpected changed fingerprint key: %+v", change)
	}
	if change.Before.Version != "1.20" || change.After.Version != "1.24" {
		t.Fatalf("unexpected changed fingerprint tuple: %+v", change)
	}
}

func TestGetImportDeltaValidationAndPreviewBehavior(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	project, _ := db.CreateProject("delta-validation")
	baseID, err := insertDeltaImportFixture(db, project.ID, "base.xml",
		[]HostObservation{{ProjectID: project.ID, IPAddress: "192.0.2.1", InScope: true, HostState: "up"}},
		[]PortObservation{},
	)
	if err != nil {
		t.Fatalf("insert base import: %v", err)
	}
	targetID, err := insertDeltaImportFixture(db, project.ID, "target.xml",
		[]HostObservation{{ProjectID: project.ID, IPAddress: "192.0.2.2", InScope: true, HostState: "up"}},
		[]PortObservation{},
	)
	if err != nil {
		t.Fatalf("insert target import: %v", err)
	}

	resp, err := db.GetImportDelta(project.ID, baseID, targetID, DeltaOptions{PreviewSize: 1, IncludeLists: false})
	if err != nil {
		t.Fatalf("get import delta: %v", err)
	}
	if resp.Lists != nil {
		t.Fatalf("expected no lists when include_lists=false")
	}

	_, err = db.GetImportDelta(project.ID, 999999, targetID, DeltaOptions{})
	if !errors.Is(err, ErrDeltaImportNotFound) {
		t.Fatalf("expected ErrDeltaImportNotFound for missing base import, got %v", err)
	}
}

func insertDeltaImportFixture(db *DB, projectID int64, filename string, hosts []HostObservation, ports []PortObservation) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(ScanImport{ProjectID: projectID, Filename: filename})
	if err != nil {
		return 0, err
	}

	for _, host := range hosts {
		host.ScanImportID = record.ID
		host.ProjectID = projectID
		if _, err := tx.InsertHostObservation(host); err != nil {
			return 0, err
		}
	}
	for _, port := range ports {
		port.ScanImportID = record.ID
		port.ProjectID = projectID
		if _, err := tx.InsertPortObservation(port); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return record.ID, nil
}
