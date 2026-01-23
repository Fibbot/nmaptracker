package web

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/export"
)

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/projects", http.StatusFound)
}

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	projects, err := s.DB.ListProjects()
	if err != nil {
		http.Error(w, "failed to list projects", http.StatusInternalServerError)
		return
	}
	render(w, r, ProjectsListPage(projects))
}

func (s *Server) handleProjectDashboard(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	project, found, err := s.DB.GetProjectByID(projectID)
	if err != nil {
		http.Error(w, "failed to load project", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	stats, err := s.DB.GetDashboardStats(projectID)
	if err != nil {
		http.Error(w, "failed to load dashboard stats", http.StatusInternalServerError)
		return
	}

	totalFlagged := stats.WorkStatus.Flagged + stats.WorkStatus.InProgress + stats.WorkStatus.Done + stats.WorkStatus.ParkingLot
	progress := progressPercent(stats.WorkStatus.Done, totalFlagged)

	render(w, r, DashboardPage(project, stats, totalFlagged, progress))
}

func (s *Server) handleProjectHosts(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	project, found, err := s.DB.GetProjectByID(projectID)
	if err != nil {
		http.Error(w, "failed to load project", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "project not found", http.StatusNotFound)
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
		http.Error(w, "invalid in_scope filter", http.StatusBadRequest)
		return
	}
	statusFilters := parseStatusFilters(filters.Status)
	sortBy := normalizeSort(filters.Sort)
	dir := normalizeDir(filters.Dir)

	page, pageSize := parsePagination(filters.Page, filters.Size)
	offset := (page - 1) * pageSize
	items, total, err := s.DB.ListHostsWithSummaryPaged(projectID, inScope, statusFilters, sortBy, dir, pageSize, offset)
	if err != nil {
		http.Error(w, "failed to load hosts", http.StatusInternalServerError)
		return
	}

	if filters.Subnet != "" {
		prefix, err := netip.ParsePrefix(filters.Subnet)
		if err != nil {
			http.Error(w, "invalid subnet filter", http.StatusBadRequest)
			return
		}
		filtered := items[:0]
		for _, item := range items {
			addr, err := netip.ParseAddr(item.IPAddress)
			if err != nil {
				continue
			}
			if prefix.Contains(addr) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	filters.Sort = sortBy
	filters.Dir = dir
	filters.Page = strconv.Itoa(page)
	filters.Size = strconv.Itoa(pageSize)
	if isHTMXRequest(r) {
		render(w, r, HostsTablePartial(project, filters, items, total))
		return
	}
	render(w, r, HostsPage(project, filters, items, total))
}

func (s *Server) handleHostDetail(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}
	project, found, err := s.DB.GetProjectByID(projectID)
	if err != nil {
		http.Error(w, "failed to load project", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil {
		http.Error(w, "failed to load host", http.StatusInternalServerError)
		return
	}
	if !found || host.ProjectID != projectID {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	ports, err := s.DB.ListPorts(hostID)
	if err != nil {
		http.Error(w, "failed to load ports", http.StatusInternalServerError)
		return
	}

	stateFilters, err := parsePortStates(r.URL.Query()["state"])
	if err != nil {
		http.Error(w, "invalid port state filter", http.StatusBadRequest)
		return
	}
	if len(stateFilters) == 0 {
		stateFilters = map[string]bool{"open": true}
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

	render(w, r, HostDetailPage(project, host, ports, stateFilters))
}

func (s *Server) handleHostNotesUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil {
		http.Error(w, "failed to load host", http.StatusInternalServerError)
		return
	}
	if !found || host.ProjectID != projectID {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	if err := s.DB.UpdateHostNotes(hostID, notes); err != nil {
		http.Error(w, "failed to update host notes", http.StatusInternalServerError)
		return
	}
	if isHTMXRequest(r) {
		updatedHost, found, err := s.DB.GetHostByID(hostID)
		if err != nil {
			http.Error(w, "failed to load host", http.StatusInternalServerError)
			return
		}
		if !found || updatedHost.ProjectID != projectID {
			http.Error(w, "host not found", http.StatusNotFound)
			return
		}
		render(w, r, HostNotesSection(projectID, updatedHost))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts/%d", projectID, hostID), http.StatusSeeOther)
}

func (s *Server) handlePortStatusUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, portID, err := projectHostPortIDs(r)
	if err != nil {
		http.Error(w, "invalid port id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}

	port, found, err := s.DB.GetPortByID(portID)
	if err != nil {
		http.Error(w, "failed to load port", http.StatusInternalServerError)
		return
	}
	if !found || port.HostID != hostID {
		http.Error(w, "port not found", http.StatusNotFound)
		return
	}
	status := strings.TrimSpace(r.FormValue("status"))
	if !isValidWorkStatus(status) {
		http.Error(w, "invalid work status", http.StatusBadRequest)
		return
	}
	if err := s.DB.UpdateWorkStatus(portID, status); err != nil {
		http.Error(w, "failed to update status", http.StatusInternalServerError)
		return
	}
	if isHTMXRequest(r) {
		updatedPort, found, err := s.DB.GetPortByID(portID)
		if err != nil {
			http.Error(w, "failed to load port", http.StatusInternalServerError)
			return
		}
		if !found || updatedPort.HostID != hostID {
			http.Error(w, "port not found", http.StatusNotFound)
			return
		}
		render(w, r, PortRow(projectID, hostID, updatedPort))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts/%d#port-%d", projectID, hostID, portID), http.StatusSeeOther)
}

func (s *Server) handlePortNotesUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, portID, err := projectHostPortIDs(r)
	if err != nil {
		http.Error(w, "invalid port id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	port, found, err := s.DB.GetPortByID(portID)
	if err != nil {
		http.Error(w, "failed to load port", http.StatusInternalServerError)
		return
	}
	if !found || port.HostID != hostID {
		http.Error(w, "port not found", http.StatusNotFound)
		return
	}

	if err := s.DB.UpdatePortNotes(portID, notes); err != nil {
		http.Error(w, "failed to update port notes", http.StatusInternalServerError)
		return
	}
	if isHTMXRequest(r) {
		updatedPort, found, err := s.DB.GetPortByID(portID)
		if err != nil {
			http.Error(w, "failed to load port", http.StatusInternalServerError)
			return
		}
		if !found || updatedPort.HostID != hostID {
			http.Error(w, "port not found", http.StatusNotFound)
			return
		}
		render(w, r, PortRow(projectID, hostID, updatedPort))
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts/%d#port-%d", projectID, hostID, portID), http.StatusSeeOther)
}

func (s *Server) handleHostBulkStatusUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	status := strings.TrimSpace(r.FormValue("status"))
	if !isValidWorkStatus(status) {
		http.Error(w, "invalid work status", http.StatusBadRequest)
		return
	}

	host, found, err := s.DB.GetHostByID(hostID)
	if err != nil {
		http.Error(w, "failed to load host", http.StatusInternalServerError)
		return
	}
	if !found || host.ProjectID != projectID {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}

	if err := s.DB.BulkUpdateOpenByHost(hostID, status); err != nil {
		http.Error(w, "failed to update ports", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts/%d", projectID, hostID), http.StatusSeeOther)
}

func (s *Server) handlePortNumberBulkStatusUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	status := strings.TrimSpace(r.FormValue("status"))
	if !isValidWorkStatus(status) {
		http.Error(w, "invalid work status", http.StatusBadRequest)
		return
	}
	portNumber, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port_number")))
	if err != nil || portNumber < 1 || portNumber > 65535 {
		http.Error(w, "invalid port number", http.StatusBadRequest)
		return
	}

	if err := s.DB.BulkUpdateOpenByPortNumber(projectID, portNumber, status); err != nil {
		http.Error(w, "failed to update ports", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts", projectID), http.StatusSeeOther)
}

func (s *Server) handleHostListBulkStatusUpdate(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	status := strings.TrimSpace(r.FormValue("status"))
	if !isValidWorkStatus(status) {
		http.Error(w, "invalid work status", http.StatusBadRequest)
		return
	}

	filters := hostListFilters{
		Subnet:  strings.TrimSpace(r.FormValue("subnet")),
		Status:  strings.TrimSpace(r.FormValue("status_filter")),
		InScope: strings.TrimSpace(r.FormValue("in_scope")),
		Sort:    "ip",
		Dir:     "asc",
	}
	inScope, err := parseInScope(filters.InScope)
	if err != nil {
		http.Error(w, "invalid in_scope filter", http.StatusBadRequest)
		return
	}
	statusFilters := parseStatusFilters(filters.Status)
	items, err := s.DB.ListHostsWithSummary(projectID, inScope, statusFilters, filters.Sort, filters.Dir)
	if err != nil {
		http.Error(w, "failed to load hosts", http.StatusInternalServerError)
		return
	}

	if filters.Subnet != "" {
		prefix, err := netip.ParsePrefix(filters.Subnet)
		if err != nil {
			http.Error(w, "invalid subnet filter", http.StatusBadRequest)
			return
		}
		filtered := items[:0]
		for _, item := range items {
			addr, err := netip.ParseAddr(item.IPAddress)
			if err != nil {
				continue
			}
			if prefix.Contains(addr) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	hostIDs := make([]int64, 0, len(items))
	for _, item := range items {
		hostIDs = append(hostIDs, item.ID)
	}
	if err := s.DB.BulkUpdateOpenByHostIDs(projectID, hostIDs, status); err != nil {
		http.Error(w, "failed to update ports", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/projects/%d/hosts", projectID), http.StatusSeeOther)
}

func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
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
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		if err := export.ExportProjectCSV(s.DB, projectID, w); err != nil {
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid export format", http.StatusBadRequest)
		return
	}
}

func (s *Server) handleHostExport(w http.ResponseWriter, r *http.Request) {
	projectID, hostID, err := projectHostIDs(r)
	if err != nil {
		http.Error(w, "invalid host id", http.StatusBadRequest)
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
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		if err := export.ExportHostCSV(s.DB, projectID, hostID, w); err != nil {
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid export format", http.StatusBadRequest)
		return
	}
}

func (s *Server) handleProjectsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form submission", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "project name is required", http.StatusBadRequest)
		return
	}
	if _, err := s.DB.CreateProject(name); err != nil {
		http.Error(w, fmt.Sprintf("create project: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func (s *Server) handleProjectsDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	if err := s.DB.DeleteProject(id); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, "delete project failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/projects", http.StatusSeeOther)
}

func progressPercent(done, total int) int {
	if total == 0 {
		return 0
	}
	return int(float64(done) / float64(total) * 100)
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

type hostPager struct {
	Page     int
	LastPage int
	PrevURL  string
	NextURL  string
	HasPrev  bool
	HasNext  bool
	Show     bool
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

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
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
		"open":     true,
		"closed":   true,
		"filtered": true,
	}
	out := make(map[string]bool)
	for _, val := range values {
		val = strings.ToLower(strings.TrimSpace(val))
		if val == "" {
			continue
		}
		if !allowed[val] {
			return nil, fmt.Errorf("invalid state")
		}
		out[val] = true
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
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

func buildHostPager(projectID int64, filters hostListFilters, total int) hostPager {
	if total <= 0 {
		return hostPager{}
	}
	page := parseInt(filters.Page, 1)
	size := parseInt(filters.Size, 50)
	lastPage := (total + size - 1) / size
	if lastPage < 1 {
		lastPage = 1
	}
	if page > lastPage {
		page = lastPage
	}

	pager := hostPager{
		Page:     page,
		LastPage: lastPage,
		Show:     true,
	}
	if page > 1 {
		pager.HasPrev = true
		pager.PrevURL = buildHostListLink(projectID, filters, page-1)
	}
	if page < lastPage {
		pager.HasNext = true
		pager.NextURL = buildHostListLink(projectID, filters, page+1)
	}
	return pager
}

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
	case "scanned", "flagged", "in_progress", "done", "parking_lot":
		return true
	default:
		return false
	}
}
