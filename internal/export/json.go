package export

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
)

// ProjectExport captures full project data for JSON export.
type ProjectExport struct {
	Project          ProjectInfo           `json:"project"`
	ScopeDefinitions []ScopeDefinitionInfo `json:"scope_definitions"`
	ScanImports      []ScanImportInfo      `json:"scan_imports"`
	Hosts            []HostExport          `json:"hosts"`
}

// HostExportPayload captures a single host export with project metadata.
type HostExportPayload struct {
	Project ProjectInfo `json:"project"`
	Host    HostInfo    `json:"host"`
	Ports   []PortInfo  `json:"ports"`
}

type ProjectInfo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScopeDefinitionInfo struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"project_id"`
	Definition string    `json:"definition"`
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"created_at"`
}

type ScanImportInfo struct {
	ID         int64     `json:"id"`
	ProjectID  int64     `json:"project_id"`
	Filename   string    `json:"filename"`
	ImportTime time.Time `json:"import_time"`
	HostsFound int       `json:"hosts_found"`
	PortsFound int       `json:"ports_found"`
}

// HostExport captures a host with ports for export.
type HostExport struct {
	Host  HostInfo   `json:"host"`
	Ports []PortInfo `json:"ports"`
}

type HostInfo struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	IPAddress string    `json:"ip_address"`
	Hostname  string    `json:"hostname"`
	OSGuess   string    `json:"os_guess"`
	InScope   bool      `json:"in_scope"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PortInfo struct {
	ID           int64     `json:"id"`
	HostID       int64     `json:"host_id"`
	PortNumber   int       `json:"port_number"`
	Protocol     string    `json:"protocol"`
	State        string    `json:"state"`
	Service      string    `json:"service"`
	Version      string    `json:"version"`
	Product      string    `json:"product"`
	ExtraInfo    string    `json:"extra_info"`
	WorkStatus   string    `json:"work_status"`
	ScriptOutput string    `json:"script_output"`
	Notes        string    `json:"notes"`
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ExportProjectJSON writes full project data as JSON to the writer.
func ExportProjectJSON(database *db.DB, projectID int64, w io.Writer) error {
	project, found, err := database.GetProjectByID(projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	if !found {
		return fmt.Errorf("project not found")
	}
	scopes, err := database.ListScopeDefinitions(projectID)
	if err != nil {
		return fmt.Errorf("list scope definitions: %w", err)
	}
	imports, err := database.ListScanImports(projectID)
	if err != nil {
		return fmt.Errorf("list scan imports: %w", err)
	}
	hosts, err := database.ListHosts(projectID)
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	ports, err := database.ListPortsByProject(projectID)
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}

	portsByHost := make(map[int64][]PortInfo, len(hosts))
	for _, port := range ports {
		portsByHost[port.HostID] = append(portsByHost[port.HostID], toPortInfo(port))
	}

	exportHosts := make([]HostExport, 0, len(hosts))
	for _, host := range hosts {
		exportHosts = append(exportHosts, HostExport{
			Host:  toHostInfo(host),
			Ports: portsByHost[host.ID],
		})
	}

	payload := ProjectExport{
		Project:          toProjectInfo(project),
		ScopeDefinitions: toScopeInfos(scopes),
		ScanImports:      toScanImportInfos(imports),
		Hosts:            exportHosts,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// ExportHostJSON writes host data as JSON to the writer.
func ExportHostJSON(database *db.DB, projectID, hostID int64, w io.Writer) error {
	project, found, err := database.GetProjectByID(projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	if !found {
		return fmt.Errorf("project not found")
	}
	host, found, err := database.GetHostByID(hostID)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}
	if !found || host.ProjectID != projectID {
		return fmt.Errorf("host not found")
	}
	ports, err := database.ListPorts(hostID)
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}
	exportPorts := make([]PortInfo, 0, len(ports))
	for _, port := range ports {
		exportPorts = append(exportPorts, toPortInfo(port))
	}

	payload := HostExportPayload{
		Project: toProjectInfo(project),
		Host:    toHostInfo(host),
		Ports:   exportPorts,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func toProjectInfo(project db.Project) ProjectInfo {
	return ProjectInfo{
		ID:        project.ID,
		Name:      project.Name,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	}
}

func toScopeInfos(defs []db.ScopeDefinition) []ScopeDefinitionInfo {
	out := make([]ScopeDefinitionInfo, 0, len(defs))
	for _, def := range defs {
		out = append(out, ScopeDefinitionInfo{
			ID:         def.ID,
			ProjectID:  def.ProjectID,
			Definition: def.Definition,
			Type:       def.Type,
			CreatedAt:  def.CreatedAt,
		})
	}
	return out
}

func toScanImportInfos(imports []db.ScanImport) []ScanImportInfo {
	out := make([]ScanImportInfo, 0, len(imports))
	for _, imp := range imports {
		out = append(out, ScanImportInfo{
			ID:         imp.ID,
			ProjectID:  imp.ProjectID,
			Filename:   imp.Filename,
			ImportTime: imp.ImportTime,
			HostsFound: imp.HostsFound,
			PortsFound: imp.PortsFound,
		})
	}
	return out
}

func toHostInfo(host db.Host) HostInfo {
	return HostInfo{
		ID:        host.ID,
		ProjectID: host.ProjectID,
		IPAddress: host.IPAddress,
		Hostname:  host.Hostname,
		OSGuess:   host.OSGuess,
		InScope:   host.InScope,
		Notes:     host.Notes,
		CreatedAt: host.CreatedAt,
		UpdatedAt: host.UpdatedAt,
	}
}

func toPortInfo(port db.Port) PortInfo {
	return PortInfo{
		ID:           port.ID,
		HostID:       port.HostID,
		PortNumber:   port.PortNumber,
		Protocol:     port.Protocol,
		State:        port.State,
		Service:      port.Service,
		Version:      port.Version,
		Product:      port.Product,
		ExtraInfo:    port.ExtraInfo,
		WorkStatus:   port.WorkStatus,
		ScriptOutput: port.ScriptOutput,
		Notes:        port.Notes,
		LastSeen:     port.LastSeen,
		CreatedAt:    port.CreatedAt,
		UpdatedAt:    port.UpdatedAt,
	}
}
