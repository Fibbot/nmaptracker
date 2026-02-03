package export

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
)

// ExportProjectText writes a readable text summary of a project.
func ExportProjectText(database *db.DB, projectID int64, w io.Writer) error {
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

	fmt.Fprintf(w, "Project: %s\n", project.Name)
	fmt.Fprintf(w, "Exported: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05"))

	for _, host := range hosts {
		scopeStr := "OUT-SCOPE"
		if host.InScope {
			scopeStr = "IN-SCOPE"
		}
		fmt.Fprintf(w, "Host: %s (%s) [%s]\n", host.IPAddress, host.Hostname, scopeStr)
		if host.OSGuess != "" {
			fmt.Fprintf(w, "OS: %s\n", host.OSGuess)
		}
		if host.Notes != "" {
			fmt.Fprintf(w, "Notes: %s\n", host.Notes)
		}

		ports, err := database.ListPorts(host.ID)
		if err != nil {
			return fmt.Errorf("list ports: %w", err)
		}

		if len(ports) > 0 {
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "  Port\tState\tService\tVersion\tStatus")
			for _, p := range ports {
				version := p.Product
				if p.Version != "" {
					version += " " + p.Version
				}
				fmt.Fprintf(tw, "  %d/%s\t%s\t%s\t%s\t%s\n", p.PortNumber, p.Protocol, p.State, p.Service, version, p.WorkStatus)
			}
			tw.Flush()
		} else {
			fmt.Fprintln(w, "  No ports found.")
		}
		fmt.Fprintln(w, "")
	}

	return nil
}

// ExportHostText writes a readable text summary of a single host.
func ExportHostText(database *db.DB, projectID, hostID int64, w io.Writer) error {
	host, found, err := database.GetHostByID(hostID)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}
	if !found || host.ProjectID != projectID {
		return fmt.Errorf("host not found")
	}

	scopeStr := "OUT-SCOPE"
	if host.InScope {
		scopeStr = "IN-SCOPE"
	}
	fmt.Fprintf(w, "Host: %s (%s) [%s]\n", host.IPAddress, host.Hostname, scopeStr)
	if host.OSGuess != "" {
		fmt.Fprintf(w, "OS: %s\n", host.OSGuess)
	}
	if host.Notes != "" {
		fmt.Fprintf(w, "Notes: %s\n", host.Notes)
	}

	ports, err := database.ListPorts(host.ID)
	if err != nil {
		return fmt.Errorf("list ports: %w", err)
	}

	if len(ports) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Port\tState\tService\tVersion\tStatus")
		for _, p := range ports {
			version := p.Product
			if p.Version != "" {
				version += " " + p.Version
			}
			fmt.Fprintf(tw, "  %d/%s\t%s\t%s\t%s\t%s\n", p.PortNumber, p.Protocol, p.State, p.Service, version, p.WorkStatus)
		}
		tw.Flush()
	} else {
		fmt.Fprintln(w, "  No ports found.")
	}

	return nil
}
