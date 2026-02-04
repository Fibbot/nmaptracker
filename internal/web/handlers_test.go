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

func TestUpdateHostLatestScanValidationAndScoping(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, err := database.CreateProject("Scan-A")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := database.CreateProject("Scan-B")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	hostA, err := database.UpsertHost(db.Host{ProjectID: projectA.ID, IPAddress: "10.9.0.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host A: %v", err)
	}
	hostB, err := database.UpsertHost(db.Host{ProjectID: projectB.ID, IPAddress: "10.9.1.1", InScope: true})
	if err != nil {
		t.Fatalf("upsert host B: %v", err)
	}

	validBody := []byte(`{"latest_scan":"top1k"}`)
	req := httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/hosts/"+strconv.FormatInt(hostA.ID, 10)+"/latest-scan", bytes.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid latest_scan, got %d", rec.Code)
	}

	updatedHost, found, err := database.GetHostByID(hostA.ID)
	if err != nil || !found {
		t.Fatalf("get updated host: %v found=%v", err, found)
	}
	if updatedHost.LatestScan != db.HostLatestScanTop1K {
		t.Fatalf("expected latest_scan=%q, got %q", db.HostLatestScanTop1K, updatedHost.LatestScan)
	}

	invalidBody := []byte(`{"latest_scan":"not_real"}`)
	req = httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/hosts/"+strconv.FormatInt(hostA.ID, 10)+"/latest-scan", bytes.NewReader(invalidBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid latest_scan, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/hosts/"+strconv.FormatInt(hostB.ID, 10)+"/latest-scan", bytes.NewReader(validBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for host project mismatch, got %d", rec.Code)
	}
}

func TestListHostsIncludesLatestScanField(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Scan-C")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	host, err := database.UpsertHost(db.Host{
		ProjectID: project.ID,
		IPAddress: "10.10.10.10",
		InScope:   true,
	})
	if err != nil {
		t.Fatalf("upsert host: %v", err)
	}
	if err := database.UpdateHostLatestScan(host.ID, db.HostLatestScanPingSweep); err != nil {
		t.Fatalf("update latest scan: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/hosts", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Items []struct {
			ID         int64  `json:"ID"`
			IPAddress  string `json:"IPAddress"`
			LatestScan string `json:"LatestScan"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal hosts list: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected hosts list total/items: %+v", resp)
	}
	if resp.Items[0].LatestScan != db.HostLatestScanPingSweep {
		t.Fatalf("expected latest_scan=%q in hosts list, got %q", db.HostLatestScanPingSweep, resp.Items[0].LatestScan)
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

func TestListImportsIncludesIntents(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Golf")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	record, err := database.InsertScanImport(db.ScanImport{
		ProjectID:  project.ID,
		Filename:   "scan.xml",
		HostsFound: 2,
		PortsFound: 3,
	})
	if err != nil {
		t.Fatalf("insert scan import: %v", err)
	}
	if err := database.SetScanImportIntents(project.ID, record.ID, []db.ScanImportIntentInput{
		{Intent: db.IntentTop1KTCP, Source: db.IntentSourceAuto, Confidence: 0.98},
		{Intent: db.IntentVulnNSE, Source: db.IntentSourceManual, Confidence: 1.0},
	}); err != nil {
		t.Fatalf("set intents: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/imports", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Items []struct {
			ID      int64 `json:"id"`
			Intents []struct {
				Intent     string  `json:"intent"`
				Source     string  `json:"source"`
				Confidence float64 `json:"confidence"`
			} `json:"intents"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected total/items: %+v", resp)
	}
	if len(resp.Items[0].Intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(resp.Items[0].Intents))
	}
}

func TestSetImportIntentsValidationAndScoping(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, err := database.CreateProject("Hotel")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := database.CreateProject("India")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	importA, err := database.InsertScanImport(db.ScanImport{ProjectID: projectA.ID, Filename: "a.xml"})
	if err != nil {
		t.Fatalf("insert import A: %v", err)
	}
	importB, err := database.InsertScanImport(db.ScanImport{ProjectID: projectB.ID, Filename: "b.xml"})
	if err != nil {
		t.Fatalf("insert import B: %v", err)
	}

	badEnumPayload := []byte(`{"intents":[{"intent":"not_real","source":"manual","confidence":1}]}`)
	req := httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/imports/"+strconv.FormatInt(importA.ID, 10)+"/intents", bytes.NewReader(badEnumPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid enum, got %d", rec.Code)
	}

	validPayload := []byte(`{"intents":[{"intent":"top_1k_tcp","source":"manual","confidence":1}]}`)
	req = httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/imports/"+strconv.FormatInt(importB.ID, 10)+"/intents", bytes.NewReader(validPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for scoped import mismatch, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/imports/"+strconv.FormatInt(importA.ID, 10)+"/intents", bytes.NewReader(validPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	imports, err := database.ListScanImportsWithIntents(projectA.ID)
	if err != nil {
		t.Fatalf("list imports with intents: %v", err)
	}
	if len(imports) != 1 || len(imports[0].Intents) != 1 {
		t.Fatalf("unexpected imports/intents shape: %+v", imports)
	}
	if imports[0].Intents[0].Intent != db.IntentTop1KTCP || imports[0].Intents[0].Source != db.IntentSourceManual {
		t.Fatalf("unexpected stored intent: %+v", imports[0].Intents[0])
	}
}

func TestCoverageMatrixEndpoint(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Juliet")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	scopeDef, err := database.AddScopeDefinition(project.ID, "10.9.0.0/24", "cidr")
	if err != nil {
		t.Fatalf("add scope: %v", err)
	}
	h1, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.9.0.1", InScope: true})
	h2, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.9.1.5", InScope: true})

	if err := insertCoverageImportForWebTest(database, project.ID, db.IntentPingSweep, []string{h1.IPAddress}); err != nil {
		t.Fatalf("insert coverage import: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/coverage-matrix?include_missing_preview=true&missing_preview_size=5",
		nil,
	)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		SegmentMode string `json:"segment_mode"`
		Segments    []struct {
			SegmentKey string `json:"segment_key"`
			HostTotal  int    `json:"host_total"`
			Cells      map[string]struct {
				CoveredCount int `json:"covered_count"`
				MissingCount int `json:"missing_count"`
			} `json:"cells"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.SegmentMode != "scope_rules" {
		t.Fatalf("expected scope_rules mode, got %q", resp.SegmentMode)
	}

	findRow := func(key string) (struct {
		SegmentKey string `json:"segment_key"`
		HostTotal  int    `json:"host_total"`
		Cells      map[string]struct {
			CoveredCount int `json:"covered_count"`
			MissingCount int `json:"missing_count"`
		} `json:"cells"`
	}, bool) {
		for _, row := range resp.Segments {
			if row.SegmentKey == key {
				return row, true
			}
		}
		return struct {
			SegmentKey string `json:"segment_key"`
			HostTotal  int    `json:"host_total"`
			Cells      map[string]struct {
				CoveredCount int `json:"covered_count"`
				MissingCount int `json:"missing_count"`
			} `json:"cells"`
		}{}, false
	}
	scopeKey := "scope:" + strconv.FormatInt(scopeDef.ID, 10)
	scopeRow, ok := findRow(scopeKey)
	if !ok {
		t.Fatalf("missing scope row %q", scopeKey)
	}
	if scopeRow.HostTotal != 1 {
		t.Fatalf("expected 1 host in scope segment, got %d", scopeRow.HostTotal)
	}
	if scopeRow.Cells[db.IntentPingSweep].CoveredCount != 1 {
		t.Fatalf("expected ping_sweep covered host in scope segment")
	}
	unmappedRow, ok := findRow("scope:unmapped")
	if !ok {
		t.Fatalf("missing scope:unmapped row")
	}
	if unmappedRow.Cells[db.IntentPingSweep].MissingCount != 1 {
		t.Fatalf("expected one missing host in unmapped segment")
	}

	_ = h2
}

func TestCoverageMatrixMissingEndpoint(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Kilo")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = database.AddScopeDefinition(project.ID, "10.10.0.0/24", "cidr")
	if err != nil {
		t.Fatalf("add scope: %v", err)
	}
	h1, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.10.1.4", InScope: true})

	if err := insertCoverageImportForWebTest(database, project.ID, db.IntentTopUDP, []string{}); err != nil {
		t.Fatalf("insert coverage import: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/coverage-matrix/missing?segment_key=scope:unmapped&intent=top_udp&page=1&page_size=50",
		nil,
	)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Items []db.CoverageMatrixMissingHost `json:"items"`
		Total int                            `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].IPAddress != h1.IPAddress {
		t.Fatalf("unexpected missing response: %+v", resp)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/coverage-matrix/missing?segment_key=scope:unmapped&intent=nope",
		nil,
	)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid intent, got %d", rec.Code)
	}
}

func insertCoverageImportForWebTest(database *db.DB, projectID int64, intent string, ips []string) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(db.ScanImport{
		ProjectID: projectID,
		Filename:  "coverage.xml",
	})
	if err != nil {
		return err
	}
	if _, err := tx.InsertScanImportIntent(db.ScanImportIntent{
		ScanImportID: record.ID,
		Intent:       intent,
		Source:       db.IntentSourceManual,
		Confidence:   1.0,
	}); err != nil {
		return err
	}
	for _, ip := range ips {
		if _, err := tx.InsertHostObservation(db.HostObservation{
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

func TestImportDeltaEndpointValidation(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, _ := database.CreateProject("November")
	projectB, _ := database.CreateProject("Oscar")

	baseA, err := insertDeltaImportForWebTest(database, projectA.ID, "base.xml", []db.HostObservation{}, []db.PortObservation{})
	if err != nil {
		t.Fatalf("insert base import: %v", err)
	}
	targetA, err := insertDeltaImportForWebTest(database, projectA.ID, "target.xml", []db.HostObservation{}, []db.PortObservation{})
	if err != nil {
		t.Fatalf("insert target import: %v", err)
	}
	foreignImport, err := insertDeltaImportForWebTest(database, projectB.ID, "foreign.xml", []db.HostObservation{}, []db.PortObservation{})
	if err != nil {
		t.Fatalf("insert foreign import: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/delta", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing query params, got %d", rec.Code)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/delta?base_import_id="+strconv.FormatInt(baseA, 10)+"&target_import_id="+strconv.FormatInt(baseA, 10),
		nil,
	)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for same import IDs, got %d", rec.Code)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/delta?base_import_id="+strconv.FormatInt(baseA, 10)+"&target_import_id="+strconv.FormatInt(foreignImport, 10),
		nil,
	)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-project import, got %d", rec.Code)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/delta?base_import_id="+strconv.FormatInt(baseA, 10)+"&target_import_id="+strconv.FormatInt(targetA, 10),
		nil,
	)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid delta request, got %d", rec.Code)
	}
}

func TestImportDeltaEndpointResponse(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, _ := database.CreateProject("Papa")

	baseID, err := insertDeltaImportForWebTest(
		database,
		project.ID,
		"week1.xml",
		[]db.HostObservation{
			{IPAddress: "10.80.0.1", Hostname: "base-host", InScope: true, HostState: "up"},
			{IPAddress: "10.80.0.2", Hostname: "", InScope: true, HostState: "up"},
		},
		[]db.PortObservation{
			{IPAddress: "10.80.0.1", PortNumber: 443, Protocol: "tcp", State: "open", Service: "https", Product: "nginx", Version: "1.20"},
			{IPAddress: "10.80.0.2", PortNumber: 22, Protocol: "tcp", State: "open", Service: "ssh", Product: "openssh", Version: "8.0"},
		},
	)
	if err != nil {
		t.Fatalf("insert base import: %v", err)
	}
	targetID, err := insertDeltaImportForWebTest(
		database,
		project.ID,
		"week2.xml",
		[]db.HostObservation{
			{IPAddress: "10.80.0.1", Hostname: "base-host", InScope: true, HostState: "up"},
			{IPAddress: "10.80.0.3", Hostname: "new-host", InScope: true, HostState: "up"},
		},
		[]db.PortObservation{
			{IPAddress: "10.80.0.1", PortNumber: 443, Protocol: "tcp", State: "open", Service: "https", Product: "nginx", Version: "1.24"},
			{IPAddress: "10.80.0.3", PortNumber: 3389, Protocol: "tcp", State: "open|filtered", Service: "ms-wbt-server"},
		},
	)
	if err != nil {
		t.Fatalf("insert target import: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/delta?base_import_id="+strconv.FormatInt(baseID, 10)+"&target_import_id="+strconv.FormatInt(targetID, 10)+"&preview_size=50&include_lists=true",
		nil,
	)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Summary struct {
			NetNewHosts                int `json:"net_new_hosts"`
			DisappearedHosts           int `json:"disappeared_hosts"`
			NetNewOpenExposures        int `json:"net_new_open_exposures"`
			DisappearedOpenExposures   int `json:"disappeared_open_exposures"`
			ChangedServiceFingerprints int `json:"changed_service_fingerprints"`
		} `json:"summary"`
		Lists struct {
			NetNewHosts                []db.DeltaHost               `json:"net_new_hosts"`
			DisappearedHosts           []db.DeltaHost               `json:"disappeared_hosts"`
			NetNewOpenExposures        []db.DeltaExposure           `json:"net_new_open_exposures"`
			DisappearedOpenExposures   []db.DeltaExposure           `json:"disappeared_open_exposures"`
			ChangedServiceFingerprints []db.DeltaChangedFingerprint `json:"changed_service_fingerprints"`
		} `json:"lists"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Summary.NetNewHosts != 1 || resp.Summary.DisappearedHosts != 1 {
		t.Fatalf("unexpected host summary: %+v", resp.Summary)
	}
	if resp.Summary.NetNewOpenExposures != 1 || resp.Summary.DisappearedOpenExposures != 1 {
		t.Fatalf("unexpected exposure summary: %+v", resp.Summary)
	}
	if resp.Summary.ChangedServiceFingerprints != 1 {
		t.Fatalf("unexpected fingerprint summary: %+v", resp.Summary)
	}
	if len(resp.Lists.NetNewHosts) != 1 || resp.Lists.NetNewHosts[0].IPAddress != "10.80.0.3" {
		t.Fatalf("unexpected net_new_hosts list: %+v", resp.Lists.NetNewHosts)
	}
	if len(resp.Lists.ChangedServiceFingerprints) != 1 || resp.Lists.ChangedServiceFingerprints[0].After.Version != "1.24" {
		t.Fatalf("unexpected changed_service_fingerprints list: %+v", resp.Lists.ChangedServiceFingerprints)
	}
}

func TestBaselineEndpointsCRUDAndEvaluate(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Quebec")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if _, err := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.1", Hostname: "inside", InScope: true}); err != nil {
		t.Fatalf("insert host 10.0.0.1: %v", err)
	}
	if _, err := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.9", Hostname: "outside-a", InScope: false}); err != nil {
		t.Fatalf("insert host 10.0.0.9: %v", err)
	}
	if _, err := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.0.0.10", Hostname: "outside-b", InScope: true}); err != nil {
		t.Fatalf("insert host 10.0.0.10: %v", err)
	}

	postBody := []byte(`{"definitions":["10.0.0.0/30","10.0.0.8","10.0.0.8"]}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/baseline", bytes.NewReader(postBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	var postResp struct {
		Added int `json:"added"`
		Items []struct {
			ID         int64  `json:"id"`
			Definition string `json:"definition"`
			Type       string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("unmarshal post baseline: %v", err)
	}
	if postResp.Added != 2 || len(postResp.Items) != 2 {
		t.Fatalf("unexpected baseline add response: %+v", postResp)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/baseline", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var listResp struct {
		Items []struct {
			ID         int64  `json:"id"`
			ProjectID  int64  `json:"project_id"`
			Definition string `json:"definition"`
			Type       string `json:"type"`
			CreatedAt  string `json:"created_at"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list baseline: %v", err)
	}
	if listResp.Total != 2 || len(listResp.Items) != 2 {
		t.Fatalf("unexpected baseline list response: %+v", listResp)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/baseline/evaluate", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var evalResp struct {
		GeneratedAt string `json:"generated_at"`
		ProjectID   int64  `json:"project_id"`
		Summary     struct {
			ExpectedTotal              int `json:"expected_total"`
			ObservedTotal              int `json:"observed_total"`
			ExpectedButUnseen          int `json:"expected_but_unseen"`
			SeenButOutOfScope          int `json:"seen_but_out_of_scope"`
			MarkedInScopeOutOfScope    int `json:"seen_but_out_of_scope_and_marked_in_scope"`
			MarkedOutOfScopeOutOfScope int `json:"seen_but_out_of_scope_and_marked_out_scope"`
		} `json:"summary"`
		Lists struct {
			ExpectedButUnseen []string `json:"expected_but_unseen"`
			SeenButOutOfScope []struct {
				HostID    int64  `json:"host_id"`
				IPAddress string `json:"ip_address"`
				Hostname  string `json:"hostname"`
				InScope   bool   `json:"in_scope"`
			} `json:"seen_but_out_of_scope"`
		} `json:"lists"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &evalResp); err != nil {
		t.Fatalf("unmarshal evaluate baseline: %v", err)
	}
	if evalResp.ProjectID != project.ID {
		t.Fatalf("unexpected project id in eval response: %d", evalResp.ProjectID)
	}
	if evalResp.Summary.ExpectedTotal != 5 || evalResp.Summary.ObservedTotal != 3 {
		t.Fatalf("unexpected baseline eval totals: %+v", evalResp.Summary)
	}
	if evalResp.Summary.ExpectedButUnseen != 4 || evalResp.Summary.SeenButOutOfScope != 2 {
		t.Fatalf("unexpected baseline eval diff summary: %+v", evalResp.Summary)
	}
	if evalResp.Summary.MarkedInScopeOutOfScope != 1 || evalResp.Summary.MarkedOutOfScopeOutOfScope != 1 {
		t.Fatalf("unexpected baseline eval classification summary: %+v", evalResp.Summary)
	}
	if len(evalResp.Lists.ExpectedButUnseen) != 4 || evalResp.Lists.ExpectedButUnseen[0] != "10.0.0.0" {
		t.Fatalf("unexpected expected_but_unseen list: %+v", evalResp.Lists.ExpectedButUnseen)
	}
	if len(evalResp.Lists.SeenButOutOfScope) != 2 || evalResp.Lists.SeenButOutOfScope[0].IPAddress != "10.0.0.9" {
		t.Fatalf("unexpected seen_but_out_of_scope list: %+v", evalResp.Lists.SeenButOutOfScope)
	}

	req = httptest.NewRequest(http.MethodDelete, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/baseline/"+strconv.FormatInt(listResp.Items[0].ID, 10), nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/baseline", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after delete, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list baseline after delete: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Items) != 1 {
		t.Fatalf("expected 1 baseline after delete, got %+v", listResp)
	}
}

func TestBaselineEndpointsValidationAndScoping(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	projectA, err := database.CreateProject("Romeo")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	projectB, err := database.CreateProject("Sierra")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}

	_, inserted, err := database.BulkAddExpectedAssetBaselines(projectB.ID, []string{"10.99.0.0/24"})
	if err != nil {
		t.Fatalf("seed baseline for project B: %v", err)
	}
	if len(inserted) != 1 {
		t.Fatalf("expected one inserted baseline row, got %d", len(inserted))
	}

	req := httptest.NewRequest(http.MethodDelete, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/baseline/"+strconv.FormatInt(inserted[0].ID, 10), nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for scoped delete mismatch, got %d", rec.Code)
	}

	invalidBodies := []string{
		`{"definitions":["not-an-ip"]}`,
		`{"definitions":["10.0.0.0/15"]}`,
		`{"definitions":["2001:db8::1"]}`,
	}
	for _, payload := range invalidBodies {
		req = httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/projects/"+strconv.FormatInt(projectA.ID, 10)+"/baseline", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid baseline payload %s, got %d", payload, rec.Code)
		}
	}
}

func TestServiceQueueEndpointValidation(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Tango")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing campaign, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services?campaign=nope", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown campaign, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services?campaign=smb&page=0", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid page, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services?campaign=smb&page_size=0", nil)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid page_size, got %d", rec.Code)
	}
}

func TestServiceQueueEndpointResponseAndPagination(t *testing.T) {
	database, server := newTestServer(t)
	defer database.Close()

	project, err := database.CreateProject("Uniform")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	h1, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.40.0.1", Hostname: "alpha", InScope: true})
	h2, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.40.0.2", Hostname: "beta", InScope: true})
	h3, _ := database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.40.0.3", Hostname: "charlie", InScope: true})
	_, _ = database.UpsertHost(db.Host{ProjectID: project.ID, IPAddress: "10.40.0.4", Hostname: "out", InScope: false})

	if _, err := database.UpsertPort(db.Port{
		HostID:     h1.ID,
		PortNumber: 445,
		Protocol:   "tcp",
		State:      "open",
		Service:    "microsoft-ds",
		WorkStatus: "scanned",
	}); err != nil {
		t.Fatalf("insert h1 port 445: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     h1.ID,
		PortNumber: 139,
		Protocol:   "tcp",
		State:      "open|filtered",
		Service:    "netbios-ssn",
		WorkStatus: "flagged",
	}); err != nil {
		t.Fatalf("insert h1 port 139: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     h2.ID,
		PortNumber: 9000,
		Protocol:   "tcp",
		State:      "open",
		Service:    "smb",
		WorkStatus: "done",
	}); err != nil {
		t.Fatalf("insert h2 smb service port: %v", err)
	}
	if _, err := database.UpsertPort(db.Port{
		HostID:     h3.ID,
		PortNumber: 445,
		Protocol:   "tcp",
		State:      "open",
		Service:    "microsoft-ds",
		WorkStatus: "flagged",
	}); err != nil {
		t.Fatalf("insert h3 port 445: %v", err)
	}

	import1, err := insertServiceQueueImportForWebTest(database, project.ID, []string{h1.IPAddress})
	if err != nil {
		t.Fatalf("insert import1: %v", err)
	}
	import2, err := insertServiceQueueImportForWebTest(database, project.ID, []string{h3.IPAddress})
	if err != nil {
		t.Fatalf("insert import2: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services?campaign=smb&page=1&page_size=2",
		nil,
	)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Campaign        string  `json:"campaign"`
		TotalHosts      int     `json:"total_hosts"`
		Page            int     `json:"page"`
		PageSize        int     `json:"page_size"`
		SourceImportIDs []int64 `json:"source_import_ids"`
		Items           []struct {
			HostID        int64  `json:"host_id"`
			IPAddress     string `json:"ip_address"`
			Hostname      string `json:"hostname"`
			StatusSummary struct {
				Scanned    int `json:"scanned"`
				Flagged    int `json:"flagged"`
				InProgress int `json:"in_progress"`
				Done       int `json:"done"`
			} `json:"status_summary"`
			MatchingPorts []struct {
				PortID     int64  `json:"port_id"`
				PortNumber int    `json:"port_number"`
				Protocol   string `json:"protocol"`
				State      string `json:"state"`
				Service    string `json:"service"`
				WorkStatus string `json:"work_status"`
				LastSeen   string `json:"last_seen"`
			} `json:"matching_ports"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal service queue response: %v", err)
	}
	if resp.Campaign != "smb" || resp.TotalHosts != 3 || resp.Page != 1 || resp.PageSize != 2 {
		t.Fatalf("unexpected envelope: %+v", resp)
	}
	if len(resp.Items) != 2 || resp.Items[0].IPAddress != h1.IPAddress || resp.Items[1].IPAddress != h2.IPAddress {
		t.Fatalf("unexpected first page items: %+v", resp.Items)
	}
	if len(resp.Items[0].MatchingPorts) != 2 {
		t.Fatalf("expected host1 to have 2 matching ports, got %d", len(resp.Items[0].MatchingPorts))
	}
	if resp.Items[0].StatusSummary.Scanned != 1 || resp.Items[0].StatusSummary.Flagged != 1 {
		t.Fatalf("unexpected host1 status summary: %+v", resp.Items[0].StatusSummary)
	}
	if resp.Items[1].StatusSummary.Done != 1 {
		t.Fatalf("unexpected host2 status summary: %+v", resp.Items[1].StatusSummary)
	}
	if len(resp.SourceImportIDs) != 1 || resp.SourceImportIDs[0] != import1 {
		t.Fatalf("unexpected first page source_import_ids: %+v", resp.SourceImportIDs)
	}

	req = httptest.NewRequest(
		http.MethodGet,
		"http://localhost:8080/api/projects/"+strconv.FormatInt(project.ID, 10)+"/queues/services?campaign=smb&page=2&page_size=2",
		nil,
	)
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for second page, got %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal second page service queue response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].IPAddress != h3.IPAddress {
		t.Fatalf("unexpected second page items: %+v", resp.Items)
	}
	if resp.Items[0].StatusSummary.Flagged != 1 {
		t.Fatalf("unexpected host3 status summary: %+v", resp.Items[0].StatusSummary)
	}
	if len(resp.SourceImportIDs) != 1 || resp.SourceImportIDs[0] != import2 {
		t.Fatalf("unexpected second page source_import_ids: %+v", resp.SourceImportIDs)
	}
}

func insertServiceQueueImportForWebTest(database *db.DB, projectID int64, ips []string) (int64, error) {
	tx, err := database.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(db.ScanImport{
		ProjectID: projectID,
		Filename:  "service-queue.xml",
	})
	if err != nil {
		return 0, err
	}
	for _, ip := range ips {
		if _, err := tx.InsertHostObservation(db.HostObservation{
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

func insertDeltaImportForWebTest(database *db.DB, projectID int64, filename string, hosts []db.HostObservation, ports []db.PortObservation) (int64, error) {
	tx, err := database.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	record, err := tx.InsertScanImport(db.ScanImport{ProjectID: projectID, Filename: filename})
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
