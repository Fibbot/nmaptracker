package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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

func TestCSRFGuard(t *testing.T) {
	t.Run("rejects invalid origin", func(t *testing.T) {
		database, server := newTestServer(t)
		defer database.Close()

		body := bytes.NewBufferString(`{"name":"Alpha"}`)
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://evil.com")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("allows local origin", func(t *testing.T) {
		database, server := newTestServer(t)
		defer database.Close()

		body := bytes.NewBufferString(`{"name":"Bravo"}`)
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost:8080")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})

	t.Run("allows empty origin", func(t *testing.T) {
		database, server := newTestServer(t)
		defer database.Close()

		body := bytes.NewBufferString(`{"name":"Charlie"}`)
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		server.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
	})
}

func TestBulkStatusScoped(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, err := database.CreateProject("Alpha")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectB, err := database.CreateProject("Bravo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	hostA, err := database.UpsertHost(db.Host{ProjectID: projectA.ID, IPAddress: "10.0.0.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{ProjectID: projectB.ID, IPAddress: "10.0.1.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	portA, err := database.UpsertPort(db.Port{HostID: hostA.ID, PortNumber: 80, Protocol: "tcp", State: "open", WorkStatus: "scanned"})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	portB, err := database.UpsertPort(db.Port{HostID: hostB.ID, PortNumber: 443, Protocol: "tcp", State: "open", WorkStatus: "scanned"})
	if err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	payload := map[string]interface{}{
		"ids":    []int64{portA.ID, portB.ID},
		"status": "done",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/ports/bulk-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	updatedA, _, err := database.GetPortByID(portA.ID)
	if err != nil {
		t.Fatalf("get port A: %v", err)
	}
	if updatedA.WorkStatus != "done" {
		t.Fatalf("expected port A status done, got %s", updatedA.WorkStatus)
	}

	updatedB, _, err := database.GetPortByID(portB.ID)
	if err != nil {
		t.Fatalf("get port B: %v", err)
	}
	if updatedB.WorkStatus != "scanned" {
		t.Fatalf("expected port B status unchanged, got %s", updatedB.WorkStatus)
	}
}

func TestScopeDeleteScoped(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, err := database.CreateProject("Alpha")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectB, err := database.CreateProject("Bravo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	scopeDef, err := database.AddScopeDefinition(projectB.ID, "10.0.0.0/24", "cidr")
	if err != nil {
		t.Fatalf("add scope: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/scope/"+strconv.FormatInt(scopeDef.ID, 10), nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	defs, err := database.ListScopeDefinitions(projectB.ID)
	if err != nil {
		t.Fatalf("list scope: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected scope to remain, got %d", len(defs))
	}
}

func TestHostSubnetPagination(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Delta")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	_, err = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	_, err = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.2", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	_, err = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.1.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/hosts?subnet=10.0.0.0/24&page=1&page_size=1", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Items []db.HostListItem `json:"items"`
		Total int               `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/hosts?subnet=10.0.0.0/24&page=2&page_size=1", nil)
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal page 2: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item on page 2, got %d", len(resp.Items))
	}
}

func TestProjectPortsPagination(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Echo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	host, err := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.5", InScope: true})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}

	if _, err := database.UpsertPort(db.Port{HostID: host.ID, PortNumber: 80, Protocol: "tcp", State: "open", WorkStatus: "scanned"}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{HostID: host.ID, PortNumber: 443, Protocol: "tcp", State: "open", WorkStatus: "done"}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{HostID: host.ID, PortNumber: 53, Protocol: "udp", State: "closed", WorkStatus: "scanned"}); err != nil {
		t.Fatalf("upsert port: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/ports/all?page=1&page_size=2", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Items    []db.ProjectPort `json:"items"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/ports/all?page=1&page_size=10&state=open", nil)
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal open: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected open total 2, got %d", resp.Total)
	}
}

func TestImportSkipsInvalidHosts(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Foxtrot")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	xmlPayload := `<?xml version="1.0"?>
<nmaprun>
  <host>
    <address addr="192.0.2.10" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http"/>
      </port>
    </ports>
  </host>
  <host>
    <address addr="999.999.999.999" addrtype="ipv4"/>
  </host>
</nmaprun>`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "scan.xml")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(xmlPayload)); err != nil {
		t.Fatalf("write xml: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp["hosts_imported"].(float64) != 1 {
		t.Fatalf("expected hosts_imported 1, got %v", resp["hosts_imported"])
	}
	if resp["hosts_skipped"].(float64) != 1 {
		t.Fatalf("expected hosts_skipped 1, got %v", resp["hosts_skipped"])
	}
}
