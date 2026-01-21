package importer

import (
	"fmt"
	"io"
	"os"
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

// ParseXML parses nmap XML from a reader into Observations.
func ParseXML(r io.Reader) (Observations, error) {
	return parseXML(r)
}

// ImportObservations merges parsed observations into the DB for a project.
// It is additive: ports are never deleted; service/version/product/extrainfo
// fields are enriched when the new scan provides non-empty values. work_status
// is preserved for existing ports. last_seen is updated to the provided time.
func ImportObservations(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, obs Observations, now time.Time) (db.ScanImport, error) {
	tx, err := database.Begin()
	if err != nil {
		return db.ScanImport{}, err
	}
	defer tx.Rollback()

	stats := db.ScanImport{
		ProjectID:  projectID,
		Filename:   filename,
		HostsFound: len(obs.Hosts),
	}
	for _, h := range obs.Hosts {
		stats.PortsFound += len(h.Ports)
	}
	record, err := tx.InsertScanImport(stats)
	if err != nil {
		return db.ScanImport{}, err
	}

	for _, hObs := range obs.Hosts {
		inScope, err := matcher.InScope(hObs.IPAddress)
		if err != nil {
			return db.ScanImport{}, fmt.Errorf("scope check %s: %w", hObs.IPAddress, err)
		}

		existingHost, _, err := tx.GetHostByIP(projectID, hObs.IPAddress)
		if err != nil {
			return db.ScanImport{}, err
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
			return db.ScanImport{}, err
		}

		for _, pObs := range hObs.Ports {
			existingPort, _, err := tx.GetPortByKey(upsertedHost.ID, pObs.PortNumber, pObs.Protocol)
			if err != nil {
				return db.ScanImport{}, err
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
				return db.ScanImport{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return db.ScanImport{}, err
	}
	return record, nil
}

func pickNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
