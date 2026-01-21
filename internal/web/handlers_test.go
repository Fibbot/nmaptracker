package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/testutil"
)

func newTestServer(t *testing.T) (*db.DB, *Server) {
	t.Helper()
	dir := testutil.TempDir(t)
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return database, NewServer(database)
}

func TestProjectsListHandler(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Projects") {
		t.Fatalf("expected response body to contain Projects header")
	}
}

func TestProjectsCreateHandler(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	form := url.Values{}
	form.Set("name", "Red Team")
	req := httptest.NewRequest(http.MethodPost, "/projects", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/projects" {
		t.Fatalf("expected redirect to /projects, got %q", location)
	}

	projects, err := database.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "Red Team" {
		t.Fatalf("expected created project, got %#v", projects)
	}
}

func TestProjectDashboardHandler(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Echo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	hostA, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.0.1",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.0.2",
		InScope:   false,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	if _, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "done",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "flagged",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 443,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "in_progress",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 53,
		Protocol:   "udp",
		State:      "open",
		WorkStatus: "parking_lot",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/projects/1", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, expected := range []string{
		"Total hosts</p><p class=\"stat-value\">2",
		"In scope</p><p class=\"stat-value\">1",
		"Out of scope</p><p class=\"stat-value\">1",
		"Progress: <strong>1 / 4</strong> (25%)",
		"Scanned</p><p class=\"stat-value\">0",
		"Flagged</p><p class=\"stat-value\">1",
		"In progress</p><p class=\"stat-value\">1",
		"Done</p><p class=\"stat-value\">1",
		"Parking lot</p><p class=\"stat-value\">1",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %q", expected)
		}
	}
}

func TestProjectHostsHandlerFiltersAndSort(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Kilo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	hostA, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.0.10",
		Hostname:  "alpha",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.0.1.20",
		Hostname:  "bravo",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostC, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "192.168.1.5",
		Hostname:  "charlie",
		InScope:   false,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	if _, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "flagged",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "done",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 443,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "done",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     hostC.ID,
		PortNumber: 3389,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/projects/1/hosts?status=flagged&in_scope=true&subnet=10.0.0.0/24", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "10.0.0.10") {
		t.Fatalf("expected filtered host in response")
	}
	if strings.Contains(body, "10.0.1.20") || strings.Contains(body, "192.168.1.5") {
		t.Fatalf("unexpected hosts included in filtered response")
	}

	req = httptest.NewRequest(http.MethodGet, "/projects/1/hosts?sort=ports&dir=desc", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body = rec.Body.String()
	first := strings.Index(body, "10.0.1.20")
	second := strings.Index(body, "10.0.0.10")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected host with more ports to appear first")
	}
}

func TestProjectHostsPagination(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Pager")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, _ = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.1", InScope: true})
	_, _ = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.2", InScope: true})
	_, _ = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.3", InScope: true})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/hosts?page=2&page_size=1", project.ID), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "10.0.0.2") {
		t.Fatalf("expected page 2 to include 10.0.0.2")
	}
	if strings.Contains(body, "10.0.0.1") {
		t.Fatalf("expected page 2 to exclude 10.0.0.1")
	}
	if strings.Contains(body, "10.0.0.3") {
		t.Fatalf("expected page 2 to exclude 10.0.0.3")
	}
}

func TestHostDetailStatusAndNotesUpdates(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Lima")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.1.1.10",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	port, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	form := url.Values{}
	form.Set("notes", "host notes")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/hosts/%d/notes", project.ID, host.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedHost, found, err := database.GetHostByID(host.ID)
	if err != nil || !found {
		t.Fatalf("host lookup failed: %v", err)
	}
	if updatedHost.Notes != "host notes" {
		t.Fatalf("expected host notes to update, got %q", updatedHost.Notes)
	}

	form = url.Values{}
	form.Set("status", "flagged")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/hosts/%d/ports/%d/status", project.ID, host.ID, port.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedPort, found, err := database.GetPortByID(port.ID)
	if err != nil || !found {
		t.Fatalf("port lookup failed: %v", err)
	}
	if updatedPort.WorkStatus != "flagged" {
		t.Fatalf("expected work status to update, got %q", updatedPort.WorkStatus)
	}

	form = url.Values{}
	form.Set("notes", "port notes")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/hosts/%d/ports/%d/notes", project.ID, host.ID, port.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedPort, found, err = database.GetPortByID(port.ID)
	if err != nil || !found {
		t.Fatalf("port lookup failed: %v", err)
	}
	if updatedPort.Notes != "port notes" {
		t.Fatalf("expected port notes to update, got %q", updatedPort.Notes)
	}
}

func TestHostDetailStateFilter(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Sierra")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.2.2.2",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "closed",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 53,
		Protocol:   "udp",
		State:      "filtered",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/hosts/%d?state=open&state=filtered", project.ID, host.ID), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "22/tcp") || !strings.Contains(body, "53/udp") {
		t.Fatalf("expected open and filtered ports")
	}
	if strings.Contains(body, "80/tcp") {
		t.Fatalf("expected closed port to be filtered out")
	}
}

func TestHostDetailDefaultStateFilter(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Tango")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.3.3.3",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     host.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "closed",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/hosts/%d", project.ID, host.ID), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "22/tcp") {
		t.Fatalf("expected open port to be shown")
	}
	if strings.Contains(body, "80/tcp") {
		t.Fatalf("expected closed port to be hidden by default")
	}
}

func TestBulkStatusHandlers(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Uniform")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	hostA, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.4.4.4",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.4.5.5",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	openPort, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	closedPort, err := database.UpsertPort(db.Port{
		HostID:     hostA.ID,
		PortNumber: 80,
		Protocol:   "tcp",
		State:      "closed",
		WorkStatus: "scanned",
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	portB, err := database.UpsertPort(db.Port{
		HostID:     hostB.ID,
		PortNumber: 22,
		Protocol:   "tcp",
		State:      "open",
		WorkStatus: "scanned",
	})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	form := url.Values{}
	form.Set("status", "flagged")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/hosts/%d/bulk-status", project.ID, hostA.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedOpen, _, _ := database.GetPortByID(openPort.ID)
	updatedClosed, _, _ := database.GetPortByID(closedPort.ID)
	if updatedOpen.WorkStatus != "flagged" {
		t.Fatalf("expected open port to update, got %q", updatedOpen.WorkStatus)
	}
	if updatedClosed.WorkStatus != "scanned" {
		t.Fatalf("expected closed port to remain scanned, got %q", updatedClosed.WorkStatus)
	}

	form = url.Values{}
	form.Set("port_number", "22")
	form.Set("status", "done")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/ports/bulk-status", project.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedPortB, _, _ := database.GetPortByID(portB.ID)
	if updatedPortB.WorkStatus != "done" {
		t.Fatalf("expected port number bulk update, got %q", updatedPortB.WorkStatus)
	}

	form = url.Values{}
	form.Set("status", "in_progress")
	form.Set("subnet", "10.4.4.0/24")
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/projects/%d/hosts/bulk-status", project.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	updatedOpen, _, _ = database.GetPortByID(openPort.ID)
	updatedPortB, _, _ = database.GetPortByID(portB.ID)
	if updatedOpen.WorkStatus != "in_progress" {
		t.Fatalf("expected filtered host to update, got %q", updatedOpen.WorkStatus)
	}
	if updatedPortB.WorkStatus != "done" {
		t.Fatalf("expected non-matching host to remain done, got %q", updatedPortB.WorkStatus)
	}
}

func TestProjectExportHandler(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Victor")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/export?format=json", project.ID), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content type: %s", contentType)
	}
}

func TestHostExportHandler(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Whiskey")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	host, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.9.9.9",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/projects/%d/hosts/%d/export?format=csv", project.ID, host.ID), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/csv") {
		t.Fatalf("unexpected content type: %s", contentType)
	}
}
