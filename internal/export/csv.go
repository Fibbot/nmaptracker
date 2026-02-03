package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
)

// ExportProjectCSV writes a flattened port list with host info.
func ExportProjectCSV(database *db.DB, projectID int64, w io.Writer) error {
	project, found, err := database.GetProjectByID(projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	if !found {
		return fmt.Errorf("project not found")
	}

	hosts, err := database.ListHosts(projectID)
	if err != nil {
		return fmt.Errorf("list hosts: %w", err)
	}
	ports, err := database.ListPortsByProject(projectID)
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}

	hostByID := make(map[int64]db.Host, len(hosts))
	for _, host := range hosts {
		hostByID[host.ID] = host
	}

	writer := csv.NewWriter(w)
	if err := writer.Write(csvHeader()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, port := range ports {
		host, ok := hostByID[port.HostID]
		if !ok {
			continue
		}
		if err := writer.Write(csvRow(project, host, port)); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

// ExportHostCSV writes a flattened port list for a single host.
func ExportHostCSV(database *db.DB, projectID, hostID int64, w io.Writer) error {
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

	writer := csv.NewWriter(w)
	if err := writer.Write(csvHeader()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	ports, err := database.ListPorts(host.ID)
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}
	for _, port := range ports {
		row := csvRow(project, host, port)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func boolToString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func csvHeader() []string {
	return []string{
		"project_id",
		"project_name",
		"host_id",
		"ip_address",
		"hostname",
		"os_guess",
		"in_scope",
		"host_notes",
		"port_id",
		"port_number",
		"protocol",
		"state",
		"service",
		"version",
		"product",
		"extra_info",
		"work_status",
		"script_output",
		"port_notes",
		"last_seen",
	}
}

func csvRow(project db.Project, host db.Host, port db.Port) []string {
	return []string{
		strconv.FormatInt(project.ID, 10),
		project.Name,
		strconv.FormatInt(host.ID, 10),
		host.IPAddress,
		host.Hostname,
		host.OSGuess,
		boolToString(host.InScope),
		host.Notes,
		strconv.FormatInt(port.ID, 10),
		strconv.Itoa(port.PortNumber),
		port.Protocol,
		port.State,
		port.Service,
		port.Version,
		port.Product,
		port.ExtraInfo,
		port.WorkStatus,
		port.ScriptOutput,
		port.Notes,
		formatTime(port.LastSeen),
	}
}
