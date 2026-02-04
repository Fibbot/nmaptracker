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
