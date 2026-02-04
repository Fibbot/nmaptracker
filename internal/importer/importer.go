package importer

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/scope"
)

var (
	reTopPorts1000 = regexp.MustCompile(`(?i)--top-ports(?:=|\s+)1000(?:\D|$)`)
	reScriptVuln   = regexp.MustCompile(`(?i)--script(?:=|\s+)vuln(?:\D|$)`)
	reFullTCP      = regexp.MustCompile(`(?i)(?:^|\s)-p(?:=|\s+)?(?:t:)?1-65535(?:\s|$)`)
	rePortSelect   = regexp.MustCompile(`(?i)(?:^|\s)-p(?:=|\s+|$|[0-9t])`)
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
	HostState string
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

// ParseMetadata captures import metadata from a parsed XML file.
type ParseMetadata struct {
	NmapArgs string
}

// ImportOptions controls optional behavior during import.
type ImportOptions struct {
	ManualIntents []string
}

// SuggestedIntent represents an auto-inferred intent.
type SuggestedIntent struct {
	Intent     string
	Confidence float64
}

// ImportStats holds results of an import operation.
type ImportStats struct {
	db.ScanImport
	InScope  int
	OutScope int
	Skipped  int
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

// ParseXMLFileWithMetadata reads an nmap XML file from disk with metadata.
func ParseXMLFileWithMetadata(path string) (Observations, ParseMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return Observations{}, ParseMetadata{}, fmt.Errorf("open xml: %w", err)
	}
	defer f.Close()
	return ParseXMLWithMetadata(f)
}

// ImportXMLFile opens an XML file and streams it into the database.
func ImportXMLFile(database *db.DB, matcher *scope.Matcher, projectID int64, path string, now time.Time) (ImportStats, error) {
	return ImportXMLFileWithOptions(database, matcher, projectID, path, ImportOptions{}, now)
}

// ImportXMLFileWithOptions opens an XML file and streams it into the database.
func ImportXMLFileWithOptions(database *db.DB, matcher *scope.Matcher, projectID int64, path string, options ImportOptions, now time.Time) (ImportStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImportStats{}, fmt.Errorf("open xml: %w", err)
	}
	defer f.Close()

	return ImportXMLWithOptions(database, matcher, projectID, filepath.Base(path), f, options, now)
}

// ParseXML parses nmap XML from a reader into Observations.
func ParseXML(r io.Reader) (Observations, error) {
	obs, _, err := parseXMLWithMetadata(r)
	if err != nil {
		return Observations{}, err
	}
	return obs, nil
}

// ParseXMLWithMetadata parses nmap XML and returns extracted metadata.
func ParseXMLWithMetadata(r io.Reader) (Observations, ParseMetadata, error) {
	return parseXMLWithMetadata(r)
}

// ImportObservations merges parsed observations into the DB for a project.
func ImportObservations(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, obs Observations, now time.Time) (ImportStats, error) {
	return ImportObservationsWithOptions(database, matcher, projectID, filename, obs, ParseMetadata{}, ImportOptions{}, now)
}

// ImportObservationsWithOptions merges parsed observations into the DB for a project.
func ImportObservationsWithOptions(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, obs Observations, metadata ParseMetadata, options ImportOptions, now time.Time) (ImportStats, error) {
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
	stats.ScanImport = record

	resolvedIntents := ResolveImportIntents(options.ManualIntents, SuggestIntents(filename, metadata.NmapArgs, obs))
	if err := insertResolvedIntents(tx, stats.ScanImport.ID, resolvedIntents); err != nil {
		return ImportStats{}, err
	}

	for _, hObs := range obs.Hosts {
		if _, err := netip.ParseAddr(hObs.IPAddress); err != nil {
			return ImportStats{}, fmt.Errorf("invalid ip %q: %w", hObs.IPAddress, err)
		}

		if err := upsertHostAndObservations(tx, matcher, projectID, stats.ScanImport.ID, hObs, now, &stats); err != nil {
			return ImportStats{}, err
		}
	}

	if err := tx.UpdateScanImportCounts(stats.ScanImport.ID, stats.HostsFound, stats.PortsFound); err != nil {
		return ImportStats{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportStats{}, err
	}
	return stats, nil
}

// ImportXML streams an Nmap XML document within a single transaction.
func ImportXML(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, r io.Reader, now time.Time) (ImportStats, error) {
	return ImportXMLWithOptions(database, matcher, projectID, filename, r, ImportOptions{}, now)
}

// ImportXMLWithOptions streams an Nmap XML document within a single transaction.
func ImportXMLWithOptions(database *db.DB, matcher *scope.Matcher, projectID int64, filename string, r io.Reader, options ImportOptions, now time.Time) (ImportStats, error) {
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

	var nmapArgs string
	intentsInserted := false
	insertIntents := func() error {
		if intentsInserted {
			return nil
		}
		resolved := ResolveImportIntents(options.ManualIntents, SuggestIntents(filename, nmapArgs, Observations{}))
		if err := insertResolvedIntents(tx, stats.ScanImport.ID, resolved); err != nil {
			return err
		}
		intentsInserted = true
		return nil
	}

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
		if !ok {
			continue
		}

		if start.Name.Local == "nmaprun" {
			nmapArgs = nmapArgsFromStart(start)
			if err := insertIntents(); err != nil {
				return ImportStats{}, err
			}
			continue
		}
		if start.Name.Local != "host" {
			continue
		}
		if err := insertIntents(); err != nil {
			return ImportStats{}, err
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
		if err := upsertHostAndObservations(tx, matcher, projectID, stats.ScanImport.ID, hObs, now, &stats); err != nil {
			return ImportStats{}, err
		}
	}

	if err := insertIntents(); err != nil {
		return ImportStats{}, err
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

// SuggestIntents infers import intents from scan metadata.
func SuggestIntents(filename string, nmapArgs string, obs Observations) []SuggestedIntent {
	_ = obs

	args := strings.ToLower(strings.TrimSpace(nmapArgs))
	name := strings.ToLower(filename)

	hasSN := hasFlag(args, "-sn")
	hasTopPorts := strings.Contains(args, "--top-ports")
	hasTop1000 := reTopPorts1000.MatchString(args)
	hasSU := hasFlag(args, "-su")
	hasPortSelection := rePortSelect.MatchString(args) || strings.Contains(args, "--port")
	hasAllPorts := hasFlag(args, "-p-") || reFullTCP.MatchString(args)
	hasVulnScript := reScriptVuln.MatchString(args)
	hasArgs := args != ""
	isLikelyDefaultTop1K := hasArgs && !hasSN && !hasSU && !hasAllPorts && !hasPortSelection

	seen := make(map[string]struct{})
	var out []SuggestedIntent
	add := func(intent string, confidence float64) {
		if _, ok := seen[intent]; ok {
			return
		}
		seen[intent] = struct{}{}
		out = append(out, SuggestedIntent{Intent: intent, Confidence: confidence})
	}

	if hasSN || strings.Contains(name, "ping") {
		add(db.IntentPingSweep, 0.98)
	}
	if hasTop1000 {
		add(db.IntentTop1KTCP, 0.98)
	} else if isLikelyDefaultTop1K {
		// Default nmap TCP scans probe top 1,000 ports unless explicit port selection is provided.
		add(db.IntentTop1KTCP, 0.85)
	}
	if hasAllPorts {
		add(db.IntentAllTCP, 0.99)
	}
	if hasSU && (hasTopPorts || !hasPortSelection) {
		add(db.IntentTopUDP, 0.92)
	}
	if hasVulnScript {
		add(db.IntentVulnNSE, 0.95)
	}

	return out
}

// ResolveImportIntents merges manual and suggested intents, preferring manual values.
func ResolveImportIntents(manual []string, suggested []SuggestedIntent) []db.ScanImportIntent {
	seen := make(map[string]struct{})
	var out []db.ScanImportIntent

	for _, raw := range manual {
		intent := normalizeIntent(raw)
		if !db.ValidIntent(intent) {
			continue
		}
		if _, ok := seen[intent]; ok {
			continue
		}
		seen[intent] = struct{}{}
		out = append(out, db.ScanImportIntent{
			Intent:     intent,
			Source:     db.IntentSourceManual,
			Confidence: 1.0,
		})
	}

	for _, suggestion := range suggested {
		intent := normalizeIntent(suggestion.Intent)
		if !db.ValidIntent(intent) {
			continue
		}
		if _, ok := seen[intent]; ok {
			continue
		}
		seen[intent] = struct{}{}
		out = append(out, db.ScanImportIntent{
			Intent:     intent,
			Source:     db.IntentSourceAuto,
			Confidence: clampConfidence(suggestion.Confidence),
		})
	}

	return out
}

func upsertHostAndObservations(tx *db.Tx, matcher *scope.Matcher, projectID, scanImportID int64, hObs HostObservation, now time.Time, stats *ImportStats) error {
	inScope := matcher.InScope(hObs.IPAddress)
	if inScope {
		stats.InScope++
	} else {
		stats.OutScope++
	}

	existingHost, _, err := tx.GetHostByIP(projectID, hObs.IPAddress)
	if err != nil {
		return err
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
		return err
	}

	if _, err := tx.InsertHostObservation(db.HostObservation{
		ScanImportID: scanImportID,
		ProjectID:    projectID,
		IPAddress:    hObs.IPAddress,
		Hostname:     hObs.Hostname,
		InScope:      inScope,
		HostState:    strings.ToLower(strings.TrimSpace(hObs.HostState)),
	}); err != nil {
		return err
	}

	for _, pObs := range hObs.Ports {
		existingPort, _, err := tx.GetPortByKey(upsertedHost.ID, pObs.PortNumber, pObs.Protocol)
		if err != nil {
			return err
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
			return err
		}

		if _, err := tx.InsertPortObservation(db.PortObservation{
			ScanImportID: scanImportID,
			ProjectID:    projectID,
			IPAddress:    hObs.IPAddress,
			PortNumber:   pObs.PortNumber,
			Protocol:     pObs.Protocol,
			State:        pObs.State,
			Service:      pObs.Service,
			Version:      pObs.Version,
			Product:      pObs.Product,
			ExtraInfo:    pObs.ExtraInfo,
			ScriptOutput: pObs.ScriptOutput,
		}); err != nil {
			return err
		}
	}
	return nil
}

func insertResolvedIntents(tx *db.Tx, importID int64, intents []db.ScanImportIntent) error {
	for _, intent := range intents {
		intent.ScanImportID = importID
		if _, err := tx.InsertScanImportIntent(intent); err != nil {
			return err
		}
	}
	return nil
}

func nmapArgsFromStart(start xml.StartElement) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == "args" {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func hasFlag(args, flag string) bool {
	if args == "" {
		return false
	}
	flag = strings.ToLower(strings.TrimSpace(flag))
	for _, token := range strings.Fields(args) {
		if token == flag {
			return true
		}
		if strings.HasPrefix(token, flag+"=") {
			return true
		}
	}
	return false
}

func normalizeIntent(intent string) string {
	return strings.ToLower(strings.TrimSpace(intent))
}

func clampConfidence(conf float64) float64 {
	if conf <= 0 {
		return 0.8
	}
	if conf > 1 {
		return 1
	}
	return conf
}

func pickNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
