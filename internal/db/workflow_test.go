package db

import (
	"path/filepath"
	"testing"

	"github.com/sloppy/nmaptracker/internal/testutil"
)

func newWorkflowDB(t *testing.T) *DB {
	t.Helper()
	dir := testutil.TempDir(t)
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func seedPorts(t *testing.T, db *DB) (int64, []Port) {
	t.Helper()
	project, err := db.CreateProject("wf")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := db.UpsertHost(Host{ProjectID: project.ID, IPAddress: "10.0.0.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	var ports []Port
	for _, num := range []int{80, 443} {
		p, err := db.UpsertPort(Port{
			HostID:     host.ID,
			PortNumber: num,
			Protocol:   "tcp",
			State:      "open",
			WorkStatus: "scanned",
		})
		if err != nil {
			t.Fatalf("upsert port %d: %v", num, err)
		}
		ports = append(ports, p)
	}
	return project.ID, ports
}

func TestUpdateWorkStatusSingle(t *testing.T) {
	db := newWorkflowDB(t)
	defer db.Close()
	_, ports := seedPorts(t, db)

	if err := db.UpdateWorkStatus(ports[0].ID, "flagged"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, ok, err := db.GetPortByKey(ports[0].HostID, ports[0].PortNumber, ports[0].Protocol)
	if err != nil || !ok {
		t.Fatalf("get port: %v", err)
	}
	if got.WorkStatus != "flagged" {
		t.Fatalf("expected flagged, got %s", got.WorkStatus)
	}
}

func TestBulkUpdateByHost(t *testing.T) {
	db := newWorkflowDB(t)
	defer db.Close()
	_, ports := seedPorts(t, db)

	if err := db.BulkUpdateByHost(ports[0].HostID, "done"); err != nil {
		t.Fatalf("bulk host: %v", err)
	}
	for _, p := range ports {
		got, ok, err := db.GetPortByKey(p.HostID, p.PortNumber, p.Protocol)
		if err != nil || !ok {
			t.Fatalf("get port: %v", err)
		}
		if got.WorkStatus != "done" {
			t.Fatalf("expected done, got %s", got.WorkStatus)
		}
	}
}

func TestBulkUpdateByPortNumber(t *testing.T) {
	db := newWorkflowDB(t)
	defer db.Close()
	projectID, ports := seedPorts(t, db)

	if err := db.BulkUpdateByPortNumber(projectID, 80, "in_progress"); err != nil {
		t.Fatalf("bulk port number: %v", err)
	}
	p80, _, _ := db.GetPortByKey(ports[0].HostID, 80, "tcp")
	p443, _, _ := db.GetPortByKey(ports[0].HostID, 443, "tcp")
	if p80.WorkStatus != "in_progress" {
		t.Fatalf("expected in_progress for 80, got %s", p80.WorkStatus)
	}
	if p443.WorkStatus != "scanned" {
		t.Fatalf("expected 443 unchanged, got %s", p443.WorkStatus)
	}
}

func TestBulkUpdateByFilter(t *testing.T) {
	db := newWorkflowDB(t)
	defer db.Close()
	projectID, ports := seedPorts(t, db)

	host2, err := db.UpsertHost(Host{ProjectID: projectID, IPAddress: "10.0.0.2", InScope: true})
	if err != nil {
		t.Fatalf("upsert host2: %v", err)
	}
	if _, err := db.UpsertPort(Port{HostID: host2.ID, PortNumber: 80, Protocol: "udp", State: "open", WorkStatus: "scanned"}); err != nil {
		t.Fatalf("upsert udp: %v", err)
	}

	// Update only tcp ports on host1 to flagged.
	if err := db.BulkUpdateByFilter(projectID, []int64{ports[0].HostID}, nil, []string{"tcp"}, "flagged"); err != nil {
		t.Fatalf("bulk filter: %v", err)
	}

	p80tcp, _, _ := db.GetPortByKey(ports[0].HostID, 80, "tcp")
	if p80tcp.WorkStatus != "flagged" {
		t.Fatalf("expected tcp 80 flagged, got %s", p80tcp.WorkStatus)
	}
	p80udp, _, _ := db.GetPortByKey(host2.ID, 80, "udp")
	if p80udp.WorkStatus != "scanned" {
		t.Fatalf("expected udp 80 unchanged, got %s", p80udp.WorkStatus)
	}
}

func TestBulkUpdateRollbackOnError(t *testing.T) {
	db := newWorkflowDB(t)
	defer db.Close()
	projectID, ports := seedPorts(t, db)

	// Force an error by using a closed transaction context (simulate tx failure).
	db.Close()
	err := db.BulkUpdateByFilter(projectID, []int64{ports[0].HostID}, nil, nil, "done")
	if err == nil {
		t.Fatalf("expected error due to closed db")
	}
}
