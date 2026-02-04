package db

import (
	"fmt"
	"strings"
	"time"
)

// GapOptions controls preview list behavior.
type GapOptions struct {
	PreviewSize  int
	IncludeLists bool
}

// GapResponse provides project gap summary and preview lists.
type GapResponse struct {
	GeneratedAt time.Time  `json:"generated_at"`
	ProjectID   int64      `json:"project_id"`
	Summary     GapSummary `json:"summary"`
	Lists       *GapLists  `json:"lists,omitempty"`
}

// GapSummary captures count totals for operational gaps.
type GapSummary struct {
	InScopeNeverScanned       int `json:"in_scope_never_scanned"`
	OpenPortsScannedOrFlagged int `json:"open_ports_scanned_or_flagged"`
	NeedsPingSweep            int `json:"needs_ping_sweep"`
	NeedsTop1KTCP             int `json:"needs_top_1k_tcp"`
	NeedsAllTCP               int `json:"needs_all_tcp"`
}

// GapHost represents a host-level queue item.
type GapHost struct {
	HostID    int64  `json:"host_id"`
	IPAddress string `json:"ip_address"`
	Hostname  string `json:"hostname"`
}

// GapOpenPortRow is a flat open-port queue entry.
type GapOpenPortRow struct {
	HostID     int64  `json:"host_id"`
	IPAddress  string `json:"ip_address"`
	Hostname   string `json:"hostname"`
	PortID     int64  `json:"port_id"`
	PortNumber int    `json:"port_number"`
	Protocol   string `json:"protocol"`
	WorkStatus string `json:"work_status"`
	Service    string `json:"service"`
}

// GapOpenPortHostGroup groups open-port queue entries by host.
type GapOpenPortHostGroup struct {
	HostID    int64            `json:"host_id"`
	IPAddress string           `json:"ip_address"`
	Hostname  string           `json:"hostname"`
	Ports     []GapOpenPortRow `json:"ports"`
}

// GapLists contains preview lists and grouped open-port details.
type GapLists struct {
	InScopeNeverScanned              []GapHost              `json:"in_scope_never_scanned"`
	OpenPortsScannedOrFlagged        []GapOpenPortRow       `json:"open_ports_scanned_or_flagged"`
	OpenPortsScannedOrFlaggedGrouped []GapOpenPortHostGroup `json:"open_ports_scanned_or_flagged_grouped"`
	NeedsPingSweep                   []GapHost              `json:"needs_ping_sweep"`
	NeedsTop1KTCP                    []GapHost              `json:"needs_top_1k_tcp"`
	NeedsAllTCP                      []GapHost              `json:"needs_all_tcp"`
}

// MilestoneQueueResponse returns milestone-only queues.
type MilestoneQueueResponse struct {
	GeneratedAt time.Time        `json:"generated_at"`
	ProjectID   int64            `json:"project_id"`
	Summary     MilestoneSummary `json:"summary"`
	Lists       *MilestoneLists  `json:"lists,omitempty"`
}

// MilestoneSummary captures milestone queue totals.
type MilestoneSummary struct {
	NeedsPingSweep int `json:"needs_ping_sweep"`
	NeedsTop1KTCP  int `json:"needs_top_1k_tcp"`
	NeedsAllTCP    int `json:"needs_all_tcp"`
}

// MilestoneLists contains milestone queue previews.
type MilestoneLists struct {
	NeedsPingSweep []GapHost `json:"needs_ping_sweep"`
	NeedsTop1KTCP  []GapHost `json:"needs_top_1k_tcp"`
	NeedsAllTCP    []GapHost `json:"needs_all_tcp"`
}

// GetGapDashboard returns summary counts and optional preview lists for operational gaps.
func (db *DB) GetGapDashboard(projectID int64, opts GapOptions) (GapResponse, error) {
	opts = normalizeGapOptions(opts)

	inScopeHosts, err := db.listInScopeHosts(projectID)
	if err != nil {
		return GapResponse{}, err
	}
	neverScanned, err := db.listInScopeNeverScannedHosts(projectID)
	if err != nil {
		return GapResponse{}, err
	}
	openPorts, err := db.listOpenPortsScannedOrFlagged(projectID)
	if err != nil {
		return GapResponse{}, err
	}

	needsPing, needsTop1K, needsAll, err := db.computeMilestoneNeeds(projectID, inScopeHosts)
	if err != nil {
		return GapResponse{}, err
	}

	resp := GapResponse{
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		ProjectID:   projectID,
		Summary: GapSummary{
			InScopeNeverScanned:       len(neverScanned),
			OpenPortsScannedOrFlagged: len(openPorts),
			NeedsPingSweep:            len(needsPing),
			NeedsTop1KTCP:             len(needsTop1K),
			NeedsAllTCP:               len(needsAll),
		},
	}

	if opts.IncludeLists {
		resp.Lists = &GapLists{
			InScopeNeverScanned:              previewGapHosts(neverScanned, opts.PreviewSize),
			OpenPortsScannedOrFlagged:        previewOpenPorts(openPorts, opts.PreviewSize),
			OpenPortsScannedOrFlaggedGrouped: previewOpenPortsGrouped(openPorts, opts.PreviewSize),
			NeedsPingSweep:                   previewGapHosts(needsPing, opts.PreviewSize),
			NeedsTop1KTCP:                    previewGapHosts(needsTop1K, opts.PreviewSize),
			NeedsAllTCP:                      previewGapHosts(needsAll, opts.PreviewSize),
		}
	}

	return resp, nil
}

// GetMilestoneQueues returns milestone queue summary and optional preview lists.
func (db *DB) GetMilestoneQueues(projectID int64, opts GapOptions) (MilestoneQueueResponse, error) {
	opts = normalizeGapOptions(opts)

	inScopeHosts, err := db.listInScopeHosts(projectID)
	if err != nil {
		return MilestoneQueueResponse{}, err
	}
	needsPing, needsTop1K, needsAll, err := db.computeMilestoneNeeds(projectID, inScopeHosts)
	if err != nil {
		return MilestoneQueueResponse{}, err
	}

	resp := MilestoneQueueResponse{
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		ProjectID:   projectID,
		Summary: MilestoneSummary{
			NeedsPingSweep: len(needsPing),
			NeedsTop1KTCP:  len(needsTop1K),
			NeedsAllTCP:    len(needsAll),
		},
	}
	if opts.IncludeLists {
		resp.Lists = &MilestoneLists{
			NeedsPingSweep: previewGapHosts(needsPing, opts.PreviewSize),
			NeedsTop1KTCP:  previewGapHosts(needsTop1K, opts.PreviewSize),
			NeedsAllTCP:    previewGapHosts(needsAll, opts.PreviewSize),
		}
	}
	return resp, nil
}

func normalizeGapOptions(opts GapOptions) GapOptions {
	if opts.PreviewSize <= 0 {
		opts.PreviewSize = 10
	}
	if opts.PreviewSize > 100 {
		opts.PreviewSize = 100
	}
	return opts
}

func (db *DB) listInScopeHosts(projectID int64) ([]GapHost, error) {
	rows, err := db.Query(
		`SELECT id, ip_address, hostname
		   FROM host
		  WHERE project_id = ? AND in_scope = 1
		  ORDER BY ip_address`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list in-scope hosts: %w", err)
	}
	defer rows.Close()

	var hosts []GapHost
	for rows.Next() {
		var host GapHost
		if err := rows.Scan(&host.HostID, &host.IPAddress, &host.Hostname); err != nil {
			return nil, fmt.Errorf("scan in-scope host: %w", err)
		}
		host.Hostname = strings.TrimSpace(host.Hostname)
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list in-scope hosts rows: %w", err)
	}
	return hosts, nil
}

func (db *DB) listInScopeNeverScannedHosts(projectID int64) ([]GapHost, error) {
	rows, err := db.Query(
		`SELECT h.id, h.ip_address, h.hostname
		   FROM host h
		  WHERE h.project_id = ?
		    AND h.in_scope = 1
		    AND NOT EXISTS (
		      SELECT 1
		        FROM host_observation ho
		       WHERE ho.project_id = h.project_id
		         AND ho.ip_address = h.ip_address
		    )
		  ORDER BY h.ip_address`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list in-scope never scanned hosts: %w", err)
	}
	defer rows.Close()

	var out []GapHost
	for rows.Next() {
		var host GapHost
		if err := rows.Scan(&host.HostID, &host.IPAddress, &host.Hostname); err != nil {
			return nil, fmt.Errorf("scan in-scope never scanned host: %w", err)
		}
		host.Hostname = strings.TrimSpace(host.Hostname)
		out = append(out, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list in-scope never scanned hosts rows: %w", err)
	}
	return out, nil
}

func (db *DB) listOpenPortsScannedOrFlagged(projectID int64) ([]GapOpenPortRow, error) {
	rows, err := db.Query(
		`SELECT h.id, h.ip_address, h.hostname,
		        p.id, p.port_number, p.protocol, p.work_status, p.service
		   FROM port p
		   JOIN host h ON h.id = p.host_id
		  WHERE h.project_id = ?
		    AND h.in_scope = 1
		    AND p.state = 'open'
		    AND p.work_status IN ('scanned', 'flagged')
		  ORDER BY h.ip_address, p.port_number, p.protocol`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list open ports scanned/flagged: %w", err)
	}
	defer rows.Close()

	var out []GapOpenPortRow
	for rows.Next() {
		var row GapOpenPortRow
		if err := rows.Scan(
			&row.HostID,
			&row.IPAddress,
			&row.Hostname,
			&row.PortID,
			&row.PortNumber,
			&row.Protocol,
			&row.WorkStatus,
			&row.Service,
		); err != nil {
			return nil, fmt.Errorf("scan open ports scanned/flagged: %w", err)
		}
		row.Hostname = strings.TrimSpace(row.Hostname)
		row.Protocol = strings.ToLower(strings.TrimSpace(row.Protocol))
		row.WorkStatus = strings.ToLower(strings.TrimSpace(row.WorkStatus))
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list open ports scanned/flagged rows: %w", err)
	}
	return out, nil
}

func (db *DB) computeMilestoneNeeds(projectID int64, inScopeHosts []GapHost) ([]GapHost, []GapHost, []GapHost, error) {
	pingSatisfied, err := db.loadIntentObservedHostSet(projectID, []string{IntentPingSweep, IntentTop1KTCP, IntentAllTCP})
	if err != nil {
		return nil, nil, nil, err
	}
	top1KSatisfied, err := db.loadIntentObservedHostSet(projectID, []string{IntentTop1KTCP, IntentAllTCP})
	if err != nil {
		return nil, nil, nil, err
	}
	allSatisfied, err := db.loadIntentObservedHostSet(projectID, []string{IntentAllTCP})
	if err != nil {
		return nil, nil, nil, err
	}

	needsPing := make([]GapHost, 0)
	needsTop1K := make([]GapHost, 0)
	needsAll := make([]GapHost, 0)

	for _, host := range inScopeHosts {
		if _, ok := pingSatisfied[host.IPAddress]; !ok {
			needsPing = append(needsPing, host)
		}
		if _, ok := top1KSatisfied[host.IPAddress]; !ok {
			needsTop1K = append(needsTop1K, host)
		}
		if _, ok := allSatisfied[host.IPAddress]; !ok {
			needsAll = append(needsAll, host)
		}
	}

	return needsPing, needsTop1K, needsAll, nil
}

func (db *DB) loadIntentObservedHostSet(projectID int64, intents []string) (map[string]struct{}, error) {
	if len(intents) == 0 {
		return map[string]struct{}{}, nil
	}

	placeholders := makePlaceholders(len(intents))
	args := make([]any, 0, len(intents)+1)
	args = append(args, projectID)
	for _, intent := range intents {
		args = append(args, intent)
	}

	rows, err := db.Query(
		fmt.Sprintf(
			`SELECT DISTINCT ho.ip_address
			   FROM host_observation ho
			   JOIN scan_import_intent sii ON sii.scan_import_id = ho.scan_import_id
			  WHERE ho.project_id = ?
			    AND sii.intent IN (%s)`,
			placeholders,
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list intent observed host set: %w", err)
	}
	defer rows.Close()

	set := make(map[string]struct{})
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, fmt.Errorf("scan intent observed host set: %w", err)
		}
		set[ip] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list intent observed host set rows: %w", err)
	}
	return set, nil
}

func previewGapHosts(items []GapHost, limit int) []GapHost {
	if limit <= 0 || len(items) == 0 {
		return []GapHost{}
	}
	if len(items) <= limit {
		out := make([]GapHost, len(items))
		copy(out, items)
		return out
	}
	out := make([]GapHost, limit)
	copy(out, items[:limit])
	return out
}

func previewOpenPorts(items []GapOpenPortRow, limit int) []GapOpenPortRow {
	if limit <= 0 || len(items) == 0 {
		return []GapOpenPortRow{}
	}
	if len(items) <= limit {
		out := make([]GapOpenPortRow, len(items))
		copy(out, items)
		return out
	}
	out := make([]GapOpenPortRow, limit)
	copy(out, items[:limit])
	return out
}

func previewOpenPortsGrouped(items []GapOpenPortRow, limit int) []GapOpenPortHostGroup {
	if len(items) == 0 || limit <= 0 {
		return []GapOpenPortHostGroup{}
	}

	groups := make([]GapOpenPortHostGroup, 0)
	byHost := make(map[int64]int)
	for _, row := range items {
		idx, ok := byHost[row.HostID]
		if !ok {
			idx = len(groups)
			groups = append(groups, GapOpenPortHostGroup{
				HostID:    row.HostID,
				IPAddress: row.IPAddress,
				Hostname:  row.Hostname,
				Ports:     make([]GapOpenPortRow, 0),
			})
			byHost[row.HostID] = idx
		}
		groups[idx].Ports = append(groups[idx].Ports, row)
	}

	if len(groups) <= limit {
		return groups
	}
	return groups[:limit]
}
