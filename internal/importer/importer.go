package importer

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/scope"
)

// Observations holds parsed hosts/ports from a scan file.
type Observations struct {
	Hosts []HostObservation
}

// HostObservation represents a host and its ports.
type HostObservation struct {
	IPAddress string
	Hostname  string
	OSGuess   string
	Ports     []PortObservation
}

// PortObservation captures per-port data.
type PortObservation struct {
	PortNumber   int
	Protocol     string
	State        string
	Service      string
	Version      string
	Product      string
	ExtraInfo    string
	ScriptOutput string
}

// ParseXMLFile reads an nmap XML file from disk.
func ParseXMLFile(path string) (Observations, error) {
	f, err := os.Open(path)
	if err != nil {
		return Observations{}, fmt.Errorf("open xml: %w", err)
	}
	defer f.Close()
	return ParseXML(f)
}

// ImportXMLFile opens an XML file and streams it into the database.
func ImportXMLFile(database *db.DB, matcher *scope.Matcher, projectID int64, path string, now time.Time) (ImportStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImportStats{}, fmt.Errorf("open xml: %w", err)
	}
	defer f.Close()

	return ImportXML(database, matcher, projectID, filepath.Base(path), f, now)
}

// ParseXML parses nmap XML from a reader into Observations.
func ParseXML(r io.Reader) (Observations, error) {
	return parseXML(r)
}

// ImportObservations merges parsed observations into the DB for a project.
// It is additive: ports are never deleted; service/version/product/extrainfo
// fields are enriched when the new scan provides non-empty values. work_status
// is preserved for existing ports. last_seen is updated to the provided time.
// ImportStats holds results of an import operation.
type ImportStats struct {
	db.ScanImport
	InScope  int
	OutScope int
	Skipped  int
}

// ImportObservations merges parsed observations into the DB for a project.
func ImportObservations(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, obs Observations, now time.Time) (ImportStats, error) {
	tx, err := database.Begin()
	if err != nil {
		return ImportStats{}, err
	}
	defer tx.Rollback()

	stats := ImportStats{
		ScanImport: db.ScanImport{
			ProjectID:  projectID,
			Filename:   filename,
			HostsFound: len(obs.Hosts),
		},
	}
	for _, h := range obs.Hosts {
		stats.PortsFound += len(h.Ports)
	}
	record, err := tx.InsertScanImport(stats.ScanImport)
	if err != nil {
		return ImportStats{}, err
	}
	stats.ScanImport = record // Update with ID and timestamps

	for _, hObs := range obs.Hosts {
		// Validate IP
		if _, err := netip.ParseAddr(hObs.IPAddress); err != nil {
			return ImportStats{}, fmt.Errorf("invalid ip %q: %w", hObs.IPAddress, err)
		}

		inScope := matcher.InScope(hObs.IPAddress)
		if inScope {
			stats.InScope++
		} else {
			stats.OutScope++
		}

		existingHost, _, err := tx.GetHostByIP(projectID, hObs.IPAddress)
		if err != nil {
			return ImportStats{}, err
		}

		host := db.Host{
			ProjectID: projectID,
			IPAddress: hObs.IPAddress,
			Hostname:  pickNonEmpty(hObs.Hostname, existingHost.Hostname),
			OSGuess:   pickNonEmpty(hObs.OSGuess, existingHost.OSGuess),
			InScope:   inScope,
			Notes:     existingHost.Notes,
		}
		upsertedHost, err := tx.UpsertHost(host)
		if err != nil {
			return ImportStats{}, err
		}

		for _, pObs := range hObs.Ports {
			existingPort, _, err := tx.GetPortByKey(upsertedHost.ID, pObs.PortNumber, pObs.Protocol)
			if err != nil {
				return ImportStats{}, err
			}
			workStatus := existingPort.WorkStatus
			if workStatus == "" {
				workStatus = "scanned"
			}
			port := db.Port{
				HostID:       upsertedHost.ID,
				PortNumber:   pObs.PortNumber,
				Protocol:     pObs.Protocol,
				State:        pObs.State,
				Service:      pickNonEmpty(pObs.Service, existingPort.Service),
				Version:      pickNonEmpty(pObs.Version, existingPort.Version),
				Product:      pickNonEmpty(pObs.Product, existingPort.Product),
				ExtraInfo:    pickNonEmpty(pObs.ExtraInfo, existingPort.ExtraInfo),
				WorkStatus:   workStatus,
				ScriptOutput: pickNonEmpty(pObs.ScriptOutput, existingPort.ScriptOutput),
				Notes:        existingPort.Notes,
				LastSeen:     now,
			}
			if _, err := tx.UpsertPort(port); err != nil {
				return ImportStats{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return ImportStats{}, err
	}
	return stats, nil
}

// ImportXML streams an Nmap XML document, importing hosts and ports within a single transaction.
// Invalid or empty IPs are skipped and counted in stats.Skipped.
func ImportXML(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, r io.Reader, now time.Time) (ImportStats, error) {
	tx, err := database.Begin()
	if err != nil {
		return ImportStats{}, err
	}
	defer tx.Rollback()

	stats := ImportStats{
		ScanImport: db.ScanImport{
			ProjectID: projectID,
			Filename:  filename,
		},
	}

	record, err := tx.InsertScanImport(stats.ScanImport)
	if err != nil {
		return ImportStats{}, err
	}
	stats.ScanImport = record

	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ImportStats{}, fmt.Errorf("decode xml: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "host" {
			continue
		}

		var host nmapHost
		if err := dec.DecodeElement(&host, &start); err != nil {
			return ImportStats{}, fmt.Errorf("decode host: %w", err)
		}

		hObs := observationFromHost(host)
		if strings.TrimSpace(hObs.IPAddress) == "" {
			stats.Skipped++
			continue
		}
		addr, err := netip.ParseAddr(hObs.IPAddress)
		if err != nil || !addr.Is4() {
			stats.Skipped++
			continue
		}

		stats.HostsFound++
		stats.PortsFound += len(hObs.Ports)

		inScope := matcher.InScope(hObs.IPAddress)
		if inScope {
			stats.InScope++
		} else {
			stats.OutScope++
		}

		existingHost, _, err := tx.GetHostByIP(projectID, hObs.IPAddress)
		if err != nil {
			return ImportStats{}, err
		}

		hostRecord := db.Host{
			ProjectID: projectID,
			IPAddress: hObs.IPAddress,
			Hostname:  pickNonEmpty(hObs.Hostname, existingHost.Hostname),
			OSGuess:   pickNonEmpty(hObs.OSGuess, existingHost.OSGuess),
			InScope:   inScope,
			Notes:     existingHost.Notes,
		}
		upsertedHost, err := tx.UpsertHost(hostRecord)
		if err != nil {
			return ImportStats{}, err
		}

		for _, pObs := range hObs.Ports {
			existingPort, _, err := tx.GetPortByKey(upsertedHost.ID, pObs.PortNumber, pObs.Protocol)
			if err != nil {
				return ImportStats{}, err
			}
			workStatus := existingPort.WorkStatus
			if workStatus == "" {
				workStatus = "scanned"
			}
			port := db.Port{
				HostID:       upsertedHost.ID,
				PortNumber:   pObs.PortNumber,
				Protocol:     pObs.Protocol,
				State:        pObs.State,
				Service:      pickNonEmpty(pObs.Service, existingPort.Service),
				Version:      pickNonEmpty(pObs.Version, existingPort.Version),
				Product:      pickNonEmpty(pObs.Product, existingPort.Product),
				ExtraInfo:    pickNonEmpty(pObs.ExtraInfo, existingPort.ExtraInfo),
				WorkStatus:   workStatus,
				ScriptOutput: pickNonEmpty(pObs.ScriptOutput, existingPort.ScriptOutput),
				Notes:        existingPort.Notes,
				LastSeen:     now,
			}
			if _, err := tx.UpsertPort(port); err != nil {
				return ImportStats{}, err
			}
		}
	}

	stats.ScanImport.HostsFound = stats.HostsFound
	stats.ScanImport.PortsFound = stats.PortsFound

	if err := tx.UpdateScanImportCounts(stats.ScanImport.ID, stats.HostsFound, stats.PortsFound); err != nil {
		return ImportStats{}, err
	}

	if err := tx.Commit(); err != nil {
		return ImportStats{}, err
	}
	return stats, nil
}

func pickNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
