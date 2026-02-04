package db

import (
	"errors"
	"net/netip"
	"sort"
	"strings"
	"time"
)

var (
	ErrDeltaImportNotFound = errors.New("delta import not found")
)

// DeltaOptions controls optional preview list behavior.
type DeltaOptions struct {
	PreviewSize  int
	IncludeLists bool
}

// DeltaImportRef identifies one side of a delta comparison.
type DeltaImportRef struct {
	ID         int64     `json:"id"`
	Filename   string    `json:"filename"`
	ImportTime time.Time `json:"import_time"`
}

// ImportDeltaSummary captures aggregate change counts.
type ImportDeltaSummary struct {
	NetNewHosts                int `json:"net_new_hosts"`
	DisappearedHosts           int `json:"disappeared_hosts"`
	NetNewOpenExposures        int `json:"net_new_open_exposures"`
	DisappearedOpenExposures   int `json:"disappeared_open_exposures"`
	ChangedServiceFingerprints int `json:"changed_service_fingerprints"`
}

// DeltaHost is a host-only delta item.
type DeltaHost struct {
	IPAddress string `json:"ip_address"`
	Hostname  string `json:"hostname"`
}

// DeltaExposure is an exposure delta item.
type DeltaExposure struct {
	IPAddress  string `json:"ip_address"`
	PortNumber int    `json:"port_number"`
	Protocol   string `json:"protocol"`
	State      string `json:"state"`
	Service    string `json:"service"`
}

// DeltaFingerprintTuple is a service fingerprint snapshot.
type DeltaFingerprintTuple struct {
	Service   string `json:"service"`
	Product   string `json:"product"`
	Version   string `json:"version"`
	ExtraInfo string `json:"extra_info"`
}

// DeltaChangedFingerprint captures before/after fingerprint changes for an exposure.
type DeltaChangedFingerprint struct {
	IPAddress  string                `json:"ip_address"`
	PortNumber int                   `json:"port_number"`
	Protocol   string                `json:"protocol"`
	Before     DeltaFingerprintTuple `json:"before"`
	After      DeltaFingerprintTuple `json:"after"`
}

// ImportDeltaLists contains preview lists for each delta type.
type ImportDeltaLists struct {
	NetNewHosts                []DeltaHost               `json:"net_new_hosts"`
	DisappearedHosts           []DeltaHost               `json:"disappeared_hosts"`
	NetNewOpenExposures        []DeltaExposure           `json:"net_new_open_exposures"`
	DisappearedOpenExposures   []DeltaExposure           `json:"disappeared_open_exposures"`
	ChangedServiceFingerprints []DeltaChangedFingerprint `json:"changed_service_fingerprints"`
}

// ImportDeltaResponse is the full delta payload.
type ImportDeltaResponse struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	ProjectID    int64              `json:"project_id"`
	BaseImport   DeltaImportRef     `json:"base_import"`
	TargetImport DeltaImportRef     `json:"target_import"`
	Summary      ImportDeltaSummary `json:"summary"`
	Lists        *ImportDeltaLists  `json:"lists,omitempty"`
}

type exposureKey struct {
	ip       string
	port     int
	protocol string
}

type exposureRow struct {
	key   exposureKey
	state string
	tuple DeltaFingerprintTuple
}

// GetImportDelta compares two imports in the same project.
func (db *DB) GetImportDelta(projectID, baseImportID, targetImportID int64, opts DeltaOptions) (ImportDeltaResponse, error) {
	opts = normalizeDeltaOptions(opts)

	baseImport, found, err := db.GetScanImportForProject(projectID, baseImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}
	if !found {
		return ImportDeltaResponse{}, ErrDeltaImportNotFound
	}
	targetImport, found, err := db.GetScanImportForProject(projectID, targetImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}
	if !found {
		return ImportDeltaResponse{}, ErrDeltaImportNotFound
	}

	baseHosts, err := db.ListHostObservationsByImport(projectID, baseImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}
	targetHosts, err := db.ListHostObservationsByImport(projectID, targetImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}

	baseHostMap := toDeltaHostMap(baseHosts)
	targetHostMap := toDeltaHostMap(targetHosts)

	netNewHosts := make([]DeltaHost, 0)
	for ip, host := range targetHostMap {
		if _, ok := baseHostMap[ip]; !ok {
			netNewHosts = append(netNewHosts, host)
		}
	}
	disappearedHosts := make([]DeltaHost, 0)
	for ip, host := range baseHostMap {
		if _, ok := targetHostMap[ip]; !ok {
			disappearedHosts = append(disappearedHosts, host)
		}
	}
	sortDeltaHosts(netNewHosts)
	sortDeltaHosts(disappearedHosts)

	basePorts, err := db.ListPortObservationsByImport(projectID, baseImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}
	targetPorts, err := db.ListPortObservationsByImport(projectID, targetImportID)
	if err != nil {
		return ImportDeltaResponse{}, err
	}

	baseOpen := toOpenExposureMap(basePorts)
	targetOpen := toOpenExposureMap(targetPorts)

	netNewExposures := make([]DeltaExposure, 0)
	for key, row := range targetOpen {
		if _, ok := baseOpen[key]; ok {
			continue
		}
		netNewExposures = append(netNewExposures, DeltaExposure{
			IPAddress:  key.ip,
			PortNumber: key.port,
			Protocol:   key.protocol,
			State:      row.state,
			Service:    row.tuple.Service,
		})
	}
	disappearedExposures := make([]DeltaExposure, 0)
	for key, row := range baseOpen {
		if _, ok := targetOpen[key]; ok {
			continue
		}
		disappearedExposures = append(disappearedExposures, DeltaExposure{
			IPAddress:  key.ip,
			PortNumber: key.port,
			Protocol:   key.protocol,
			State:      row.state,
			Service:    row.tuple.Service,
		})
	}
	sortDeltaExposures(netNewExposures)
	sortDeltaExposures(disappearedExposures)

	changedFingerprints := make([]DeltaChangedFingerprint, 0)
	for key, baseRow := range baseOpen {
		targetRow, ok := targetOpen[key]
		if !ok {
			continue
		}
		if baseRow.tuple == targetRow.tuple {
			continue
		}
		changedFingerprints = append(changedFingerprints, DeltaChangedFingerprint{
			IPAddress:  key.ip,
			PortNumber: key.port,
			Protocol:   key.protocol,
			Before:     baseRow.tuple,
			After:      targetRow.tuple,
		})
	}
	sortDeltaFingerprintChanges(changedFingerprints)

	resp := ImportDeltaResponse{
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		ProjectID:   projectID,
		BaseImport: DeltaImportRef{
			ID:         baseImport.ID,
			Filename:   baseImport.Filename,
			ImportTime: baseImport.ImportTime,
		},
		TargetImport: DeltaImportRef{
			ID:         targetImport.ID,
			Filename:   targetImport.Filename,
			ImportTime: targetImport.ImportTime,
		},
		Summary: ImportDeltaSummary{
			NetNewHosts:                len(netNewHosts),
			DisappearedHosts:           len(disappearedHosts),
			NetNewOpenExposures:        len(netNewExposures),
			DisappearedOpenExposures:   len(disappearedExposures),
			ChangedServiceFingerprints: len(changedFingerprints),
		},
	}

	if opts.IncludeLists {
		resp.Lists = &ImportDeltaLists{
			NetNewHosts:                previewDeltaHosts(netNewHosts, opts.PreviewSize),
			DisappearedHosts:           previewDeltaHosts(disappearedHosts, opts.PreviewSize),
			NetNewOpenExposures:        previewDeltaExposures(netNewExposures, opts.PreviewSize),
			DisappearedOpenExposures:   previewDeltaExposures(disappearedExposures, opts.PreviewSize),
			ChangedServiceFingerprints: previewDeltaFingerprintChanges(changedFingerprints, opts.PreviewSize),
		}
	}

	return resp, nil
}

func normalizeDeltaOptions(opts DeltaOptions) DeltaOptions {
	if opts.PreviewSize <= 0 {
		opts.PreviewSize = 50
	}
	if opts.PreviewSize > 500 {
		opts.PreviewSize = 500
	}
	return opts
}

func toDeltaHostMap(items []HostObservation) map[string]DeltaHost {
	out := make(map[string]DeltaHost, len(items))
	for _, item := range items {
		ip := strings.TrimSpace(item.IPAddress)
		if ip == "" {
			continue
		}
		if _, ok := out[ip]; ok {
			continue
		}
		out[ip] = DeltaHost{
			IPAddress: ip,
			Hostname:  strings.TrimSpace(item.Hostname),
		}
	}
	return out
}

func toOpenExposureMap(items []PortObservation) map[exposureKey]exposureRow {
	out := make(map[exposureKey]exposureRow)
	for _, item := range items {
		state := strings.ToLower(strings.TrimSpace(item.State))
		if state != "open" && state != "open|filtered" {
			continue
		}
		key := exposureKey{
			ip:       strings.TrimSpace(item.IPAddress),
			port:     item.PortNumber,
			protocol: strings.ToLower(strings.TrimSpace(item.Protocol)),
		}
		if key.ip == "" || key.protocol == "" {
			continue
		}
		out[key] = exposureRow{
			key:   key,
			state: state,
			tuple: DeltaFingerprintTuple{
				Service:   normalizeFingerprintField(item.Service),
				Product:   normalizeFingerprintField(item.Product),
				Version:   normalizeFingerprintField(item.Version),
				ExtraInfo: normalizeFingerprintField(item.ExtraInfo),
			},
		}
	}
	return out
}

func normalizeFingerprintField(value string) string {
	return strings.TrimSpace(value)
}

func sortDeltaHosts(items []DeltaHost) {
	sort.Slice(items, func(i, j int) bool {
		return compareIP(items[i].IPAddress, items[j].IPAddress) < 0
	})
}

func sortDeltaExposures(items []DeltaExposure) {
	sort.Slice(items, func(i, j int) bool {
		if cmp := compareIP(items[i].IPAddress, items[j].IPAddress); cmp != 0 {
			return cmp < 0
		}
		if items[i].PortNumber != items[j].PortNumber {
			return items[i].PortNumber < items[j].PortNumber
		}
		return items[i].Protocol < items[j].Protocol
	})
}

func sortDeltaFingerprintChanges(items []DeltaChangedFingerprint) {
	sort.Slice(items, func(i, j int) bool {
		if cmp := compareIP(items[i].IPAddress, items[j].IPAddress); cmp != 0 {
			return cmp < 0
		}
		if items[i].PortNumber != items[j].PortNumber {
			return items[i].PortNumber < items[j].PortNumber
		}
		return items[i].Protocol < items[j].Protocol
	})
}

func compareIP(a, b string) int {
	addrA, errA := netip.ParseAddr(a)
	addrB, errB := netip.ParseAddr(b)
	if errA == nil && errB == nil {
		return addrA.Compare(addrB)
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func previewDeltaHosts(items []DeltaHost, limit int) []DeltaHost {
	if len(items) == 0 || limit <= 0 {
		return []DeltaHost{}
	}
	if len(items) <= limit {
		out := make([]DeltaHost, len(items))
		copy(out, items)
		return out
	}
	out := make([]DeltaHost, limit)
	copy(out, items[:limit])
	return out
}

func previewDeltaExposures(items []DeltaExposure, limit int) []DeltaExposure {
	if len(items) == 0 || limit <= 0 {
		return []DeltaExposure{}
	}
	if len(items) <= limit {
		out := make([]DeltaExposure, len(items))
		copy(out, items)
		return out
	}
	out := make([]DeltaExposure, limit)
	copy(out, items[:limit])
	return out
}

func previewDeltaFingerprintChanges(items []DeltaChangedFingerprint, limit int) []DeltaChangedFingerprint {
	if len(items) == 0 || limit <= 0 {
		return []DeltaChangedFingerprint{}
	}
	if len(items) <= limit {
		out := make([]DeltaChangedFingerprint, len(items))
		copy(out, items)
		return out
	}
	out := make([]DeltaChangedFingerprint, limit)
	copy(out, items[:limit])
	return out
}
