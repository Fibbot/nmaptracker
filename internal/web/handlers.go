package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/export"
)

// API Handlers

func (s *Server) apiListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.DB.ListProjects()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, projects, http.StatusOK)
}

func (s *Server) apiCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.badRequest(w, fmt.Errorf("project name is required"))
		return
	}

	project, err := s.DB.CreateProject(req.Name)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, project, http.StatusCreated)
}

func (s *Server) apiUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.badRequest(w, fmt.Errorf("project name is required"))
		return
	}

	if err := s.DB.UpdateProject(id, req.Name); err != nil {
		if err == sql.ErrNoRows {
			s.errorResponse(w, fmt.Errorf("project not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	project, found, err := s.DB.GetProjectByID(id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if !found {
		s.errorResponse(w, fmt.Errorf("project not found"), http.StatusNotFound)
		return
	}
	s.jsonResponse(w, project, http.StatusOK)
}

func (s *Server) apiDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	if err := s.DB.DeleteProject(id); err != nil {
		if err == sql.ErrNoRows {
			s.errorResponse(w, fmt.Errorf("project not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiGetProjectStats(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	stats, err := s.DB.GetDashboardStats(id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, stats, http.StatusOK)
}

func (s *Server) apiListHosts(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	filters := hostListFilters{
		Subnet:  strings.TrimSpace(query.Get("subnet")),
		Status:  strings.TrimSpace(query.Get("status")),
		InScope: strings.TrimSpace(query.Get("in_scope")),
		Sort:    strings.TrimSpace(query.Get("sort")),
		Dir:     strings.TrimSpace(query.Get("dir")),
		Page:    strings.TrimSpace(query.Get("page")),
		Size:    strings.TrimSpace(query.Get("page_size")),
	}

	inScope, err := parseInScope(filters.InScope)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	statusFilters := parseStatusFilters(filters.Status)
	sortBy := normalizeSort(filters.Sort)
	dir := normalizeDir(filters.Dir)
	page, pageSize := parsePagination(filters.Page, filters.Size)
	offset := (page - 1) * pageSize

	var subnetStart *int64
	var subnetEnd *int64
	if filters.Subnet != "" {
		prefix, err := netip.ParsePrefix(filters.Subnet)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid subnet"))
			return
		}
		if prefix.Addr().Is6() {
			s.badRequest(w, fmt.Errorf("ipv6 subnets are not supported"))
			return
		}
		start, end, err := ipv4PrefixRange(prefix)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid subnet"))
			return
		}
		subnetStart = &start
		subnetEnd = &end
	}

	items, total, err := s.DB.ListHostsWithSummaryPaged(projectID, inScope, statusFilters, sortBy, dir, subnetStart, subnetEnd, pageSize, offset)
	if err != nil {
		s.serverError(w, err)
		return
	}

	response := struct {
		Items []db.HostListItem `json:"items"`
		Total int               `json:"total"`
	}{
		Items: items,
		Total: total,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

func (s *Server) apiGetHost(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}
	s.jsonResponse(w, host, http.StatusOK)
}

func (s *Server) apiDeleteHost(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	// Verify host exists and matches project
	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.DeleteHost(hostID); err != nil {
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)

}

func (s *Server) apiListPorts(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	// Verify host exists
	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil || !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	ports, err := s.DB.ListPorts(hostID)
	if err != nil {
		s.serverError(w, err)
		return
	}

	stateFilters, err := parsePortStates(r.URL.Query()["state"])
	if err != nil {
		s.badRequest(w, err)
		return
	}

	if len(stateFilters) > 0 {
		filtered := ports[:0]
		for _, port := range ports {
			if stateFilters[port.State] {
				filtered = append(filtered, port)
			}
		}
		ports = filtered
	}

	s.jsonResponse(w, ports, http.StatusOK)
}

func (s *Server) apiListProjectPorts(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	page, pageSize := parsePagination(query.Get("page"), query.Get("page_size"))
	offset := (page - 1) * pageSize

	stateFilters, err := parseStateFilters(query.Get("state"))
	if err != nil {
		s.badRequest(w, err)
		return
	}
	statusFilters, err := parseWorkStatusFilters(query.Get("status"))
	if err != nil {
		s.badRequest(w, err)
		return
	}

	ports, total, err := s.DB.ListProjectPortsPaged(projectID, stateFilters, statusFilters, pageSize, offset)
	if err != nil {
		s.serverError(w, err)
		return
	}

	response := struct {
		Items    []db.ProjectPort `json:"items"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
	}{
		Items:    ports,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	s.jsonResponse(w, response, http.StatusOK)
}

func (s *Server) apiUpdateHostNotes(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}

	// Verify host ownership
	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil || !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.UpdateHostNotes(hostID, req.Notes); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiUpdateHostLatestScan(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		LatestScan string `json:"latest_scan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}

	latestScan := db.NormalizeHostLatestScan(req.LatestScan)
	if !db.ValidHostLatestScan(latestScan) {
		s.badRequest(w, fmt.Errorf("invalid latest_scan"))
		return
	}

	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil || !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.UpdateHostLatestScan(hostID, latestScan); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiUpdatePortStatus(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, portID, err := projectHostPortIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}

	if !isValidWorkStatus(req.Status) {
		s.badRequest(w, fmt.Errorf("invalid status"))
		return
	}

	// Verify port ownership
	port, found, err := s.DB.GetPortByID(portID)
	if err != nil || !found {
		s.errorResponse(w, fmt.Errorf("port not found"), http.StatusNotFound)
		return
	}
	if port.HostID != hostID {
		s.badRequest(w, fmt.Errorf("port mismatches host"))
		return
	}

	// Verify Host ownership indirectly or fetch host (skipping for perf as ID check is decent, but ideally check host->project link too)
	// Let's do it right:
	host, found, _ := s.DB.GetHostByID(hostID)
	if !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.UpdateWorkStatus(portID, req.Status); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiUpdatePortNotes(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, portID, err := projectHostPortIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}

	// Verify port/host ownership
	port, found, err := s.DB.GetPortByID(portID)
	if err != nil || !found {
		s.errorResponse(w, fmt.Errorf("port not found"), http.StatusNotFound)
		return
	}
	if port.HostID != hostID {
		s.badRequest(w, fmt.Errorf("port mismatches host"))
		return
	}
	host, found, _ := s.DB.GetHostByID(hostID)
	if !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.UpdatePortNotes(portID, req.Notes); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiHostBulkStatus(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}
	if !isValidWorkStatus(req.Status) {
		s.badRequest(w, fmt.Errorf("invalid status"))
		return
	}

	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil || !found || host.ProjectID != projectID {
		s.errorResponse(w, fmt.Errorf("host not found"), http.StatusNotFound)
		return
	}

	if err := s.DB.BulkUpdateOpenByHost(hostID, req.Status); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (s *Server) apiProjectBulkPortStatus(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	var req struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}
	if !isValidWorkStatus(req.Status) {
		s.badRequest(w, fmt.Errorf("invalid status"))
		return
	}
	if len(req.IDs) == 0 {
		s.jsonResponse(w, map[string]string{"status": "ok", "msg": "no ports selected"}, http.StatusOK)
		return
	}

	// Ideally we verify all ports belong to project, but for MVP assuming IDs are correct is acceptable risk
	// or we rely on the fact that an ID collision is unlikely to affect another project maliciously in this single user context.
	// For strict correctness, the DB query could join host/project, but the generic ID update is sufficient for now.

	if err := s.DB.BulkUpdatePortStatusesForProject(projectID, req.IDs, req.Status); err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// Export Handlers (Keep as GET returning files)

func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}

	filename := fmt.Sprintf("project-%d.%s", projectID, format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		if err := export.ExportProjectJSON(s.DB, projectID, w); err != nil {
			s.serverError(w, err)
			return
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		if err := export.ExportProjectCSV(s.DB, projectID, w); err != nil {
			s.serverError(w, err)
			return
		}
	case "txt", "text":
		w.Header().Set("Content-Type", "text/plain")
		if err := export.ExportProjectText(s.DB, projectID, w); err != nil {
			s.serverError(w, err)
			return
		}
	default:
		s.badRequest(w, fmt.Errorf("invalid export format"))
		return
	}
}

func (s *Server) handleHostExport(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}

	filename := fmt.Sprintf("host-%d.%s", hostID, format)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		if err := export.ExportHostJSON(s.DB, projectID, hostID, w); err != nil {
			s.serverError(w, err)
			return
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		if err := export.ExportHostCSV(s.DB, projectID, hostID, w); err != nil {
			s.serverError(w, err)
			return
		}
	case "txt", "text":
		w.Header().Set("Content-Type", "text/plain")
		if err := export.ExportHostText(s.DB, projectID, hostID, w); err != nil {
			s.serverError(w, err)
			return
		}
	default:
		s.badRequest(w, fmt.Errorf("invalid export format"))
		return
	}
}

// Helpers

func projectHostIDs(r *http.Request) (int64, int64, error) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	hostID, err := strconv.ParseInt(chi.URLParam(r, "hostID"), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return projectID, hostID, nil
}

func projectHostPortIDs(r *http.Request) (int64, int64, int64, error) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		return 0, 0, 0, err
	}
	portID, err := strconv.ParseInt(chi.URLParam(r, "portID"), 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return projectID, hostID, portID, nil
}

func parseProjectID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func isValidWorkStatus(status string) bool {
	switch status {
	case "scanned", "flagged", "in_progress", "done":
		return true
	default:
		return false
	}
}

type hostListFilters struct {
	Subnet  string
	Status  string
	InScope string
	Sort    string
	Dir     string
	Page    string
	Size    string
}

func parseInScope(raw string) (*bool, error) {
	if raw == "" {
		return nil, nil
	}
	switch strings.ToLower(raw) {
	case "true", "1", "yes":
		val := true
		return &val, nil
	case "false", "0", "no":
		val := false
		return &val, nil
	default:
		return nil, fmt.Errorf("invalid in_scope")
	}
}

func parseStatusFilters(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val == "" {
			continue
		}
		out = append(out, val)
	}
	return out
}

func normalizeSort(raw string) string {
	switch raw {
	case "hostname", "ports", "ip":
		return raw
	default:
		return "ip"
	}
}

func normalizeDir(raw string) string {
	switch strings.ToLower(raw) {
	case "desc":
		return "desc"
	default:
		return "asc"
	}
}

func parsePortStates(values []string) (map[string]bool, error) {
	if len(values) == 0 {
		return nil, nil
	}
	allowed := map[string]bool{
		"open":            true,
		"closed":          true,
		"filtered":        true,
		"open|filtered":   true,
		"closed|filtered": true,
		"unfiltered":      true,
	}
	out := make(map[string]bool)
	for _, val := range values {
		val = strings.ToLower(strings.TrimSpace(val))
		if val == "" {
			continue
		}
		if !allowed[val] {
			return nil, fmt.Errorf("invalid state: %s", val)
		}
		out[val] = true
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func parseStateFilters(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	lookup, err := parsePortStates(parts)
	if err != nil {
		return nil, err
	}
	if len(lookup) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(lookup))
	for key := range lookup {
		out = append(out, key)
	}
	return out, nil
}

func parseWorkStatusFilters(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	allowed := map[string]bool{
		"scanned":     true,
		"flagged":     true,
		"in_progress": true,
		"done":        true,
	}
	var out []string
	for _, part := range parts {
		val := strings.ToLower(strings.TrimSpace(part))
		if val == "" {
			continue
		}
		if !allowed[val] {
			return nil, fmt.Errorf("invalid status")
		}
		out = append(out, val)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func ipv4PrefixRange(prefix netip.Prefix) (int64, int64, error) {
	if !prefix.Addr().Is4() {
		return 0, 0, fmt.Errorf("ipv4 only")
	}
	masked := prefix.Masked().Addr()
	octets := masked.As4()
	start := uint64(octets[0])<<24 | uint64(octets[1])<<16 | uint64(octets[2])<<8 | uint64(octets[3])
	hostBits := 32 - prefix.Bits()
	if hostBits < 0 || hostBits > 32 {
		return 0, 0, fmt.Errorf("invalid prefix length")
	}
	size := uint64(1) << uint(hostBits)
	end := start + size - 1
	return int64(start), int64(end), nil
}

func parsePagination(pageRaw, sizeRaw string) (int, int) {
	page := 1
	size := 50
	if val, err := strconv.Atoi(strings.TrimSpace(pageRaw)); err == nil && val > 0 {
		page = val
	}
	if val, err := strconv.Atoi(strings.TrimSpace(sizeRaw)); err == nil && val > 0 && val <= 500 {
		size = val
	}
	return page, size
}
