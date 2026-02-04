package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

var ErrCoverageSegmentNotFound = errors.New("coverage segment not found")

// CoverageMatrixOptions controls matrix response behavior.
type CoverageMatrixOptions struct {
	IncludeMissingPreview bool
	MissingPreviewSize    int
}

// CoverageMatrixResponse is the API payload for coverage matrix views.
type CoverageMatrixResponse struct {
	GeneratedAt time.Time               `json:"generated_at"`
	ProjectID   int64                   `json:"project_id"`
	SegmentMode string                  `json:"segment_mode"`
	Intents     []string                `json:"intents"`
	Segments    []CoverageMatrixSegment `json:"segments"`
}

// CoverageMatrixSegment represents one row of the coverage matrix.
type CoverageMatrixSegment struct {
	SegmentKey   string                        `json:"segment_key"`
	SegmentLabel string                        `json:"segment_label"`
	HostTotal    int                           `json:"host_total"`
	Cells        map[string]CoverageMatrixCell `json:"cells"`
}

// CoverageMatrixCell represents one intent cell in a segment.
type CoverageMatrixCell struct {
	CoveredCount    int                         `json:"covered_count"`
	MissingCount    int                         `json:"missing_count"`
	CoveragePercent int                         `json:"coverage_percent"`
	MissingHosts    []CoverageMatrixMissingHost `json:"missing_hosts"`
}

// CoverageMatrixMissingHost identifies one host missing intent coverage.
type CoverageMatrixMissingHost struct {
	IPAddress string `json:"ip_address"`
	HostID    int64  `json:"host_id"`
	Hostname  string `json:"hostname"`
}

// CoverageMatrixMissingOptions controls missing-host drill-down pagination.
type CoverageMatrixMissingOptions struct {
	SegmentKey string
	Intent     string
	Page       int
	PageSize   int
}

type coverageSegmentHost struct {
	CoverageMatrixMissingHost
	addr netip.Addr
}

type coverageSegment struct {
	key   string
	label string
	hosts []coverageSegmentHost
}

type parsedScopeRule struct {
	definition string
	prefix     netip.Prefix
	addr       netip.Addr
	isPrefix   bool
	isAddr     bool
}

func (r parsedScopeRule) matches(addr netip.Addr) bool {
	if r.isPrefix {
		return r.prefix.Contains(addr)
	}
	if r.isAddr {
		return r.addr == addr
	}
	return false
}

// GetCoverageMatrix returns coverage by segment and intent for a project.
func (db *DB) GetCoverageMatrix(projectID int64, opts CoverageMatrixOptions) (CoverageMatrixResponse, error) {
	opts = normalizeCoverageMatrixOptions(opts)

	segments, mode, err := db.resolveCoverageSegments(projectID)
	if err != nil {
		return CoverageMatrixResponse{}, err
	}
	coveredByIntent, err := db.loadCoveredHostsByIntent(projectID)
	if err != nil {
		return CoverageMatrixResponse{}, err
	}

	intents := CoverageIntentOrder()
	response := CoverageMatrixResponse{
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		ProjectID:   projectID,
		SegmentMode: mode,
		Intents:     intents,
		Segments:    make([]CoverageMatrixSegment, 0, len(segments)),
	}

	for _, seg := range segments {
		hostTotal := len(seg.hosts)
		row := CoverageMatrixSegment{
			SegmentKey:   seg.key,
			SegmentLabel: seg.label,
			HostTotal:    hostTotal,
			Cells:        make(map[string]CoverageMatrixCell, len(intents)),
		}

		for _, intent := range intents {
			coveredIPs := coveredByIntent[intent]
			coveredCount := 0
			missing := make([]CoverageMatrixMissingHost, 0)

			for _, host := range seg.hosts {
				if _, ok := coveredIPs[host.IPAddress]; ok {
					coveredCount++
					continue
				}
				missing = append(missing, host.CoverageMatrixMissingHost)
			}

			missingCount := hostTotal - coveredCount
			coveragePercent := 0
			if hostTotal > 0 {
				coveragePercent = (coveredCount * 100) / hostTotal
			}

			preview := make([]CoverageMatrixMissingHost, 0)
			if opts.IncludeMissingPreview && len(missing) > 0 {
				max := opts.MissingPreviewSize
				if len(missing) < max {
					max = len(missing)
				}
				preview = append(preview, missing[:max]...)
			}

			row.Cells[intent] = CoverageMatrixCell{
				CoveredCount:    coveredCount,
				MissingCount:    missingCount,
				CoveragePercent: coveragePercent,
				MissingHosts:    preview,
			}
		}

		response.Segments = append(response.Segments, row)
	}

	return response, nil
}

// ListCoverageMatrixMissingHosts returns paged missing hosts for one segment/intent.
func (db *DB) ListCoverageMatrixMissingHosts(projectID int64, opts CoverageMatrixMissingOptions) ([]CoverageMatrixMissingHost, int, error) {
	intent := strings.TrimSpace(strings.ToLower(opts.Intent))
	if !ValidIntent(intent) {
		return nil, 0, fmt.Errorf("invalid intent %q", opts.Intent)
	}
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 50
	}
	if opts.PageSize > 200 {
		opts.PageSize = 200
	}

	segments, _, err := db.resolveCoverageSegments(projectID)
	if err != nil {
		return nil, 0, err
	}
	coveredByIntent, err := db.loadCoveredHostsByIntent(projectID)
	if err != nil {
		return nil, 0, err
	}

	var target *coverageSegment
	for i := range segments {
		if segments[i].key == opts.SegmentKey {
			target = &segments[i]
			break
		}
	}
	if target == nil {
		return nil, 0, ErrCoverageSegmentNotFound
	}

	covered := coveredByIntent[intent]
	missing := make([]CoverageMatrixMissingHost, 0)
	for _, host := range target.hosts {
		if _, ok := covered[host.IPAddress]; ok {
			continue
		}
		missing = append(missing, host.CoverageMatrixMissingHost)
	}

	total := len(missing)
	start := (opts.Page - 1) * opts.PageSize
	if start >= total {
		return []CoverageMatrixMissingHost{}, total, nil
	}
	end := start + opts.PageSize
	if end > total {
		end = total
	}

	items := make([]CoverageMatrixMissingHost, 0, end-start)
	items = append(items, missing[start:end]...)
	return items, total, nil
}

func normalizeCoverageMatrixOptions(opts CoverageMatrixOptions) CoverageMatrixOptions {
	if opts.MissingPreviewSize <= 0 {
		opts.MissingPreviewSize = 5
	}
	if opts.MissingPreviewSize > 50 {
		opts.MissingPreviewSize = 50
	}
	return opts
}

func (db *DB) resolveCoverageSegments(projectID int64) ([]coverageSegment, string, error) {
	hosts, err := db.listCoverageInScopeHosts(projectID)
	if err != nil {
		return nil, "", err
	}

	scopeDefs, err := db.ListScopeDefinitions(projectID)
	if err != nil {
		return nil, "", fmt.Errorf("list scope definitions: %w", err)
	}
	if len(scopeDefs) > 0 {
		segments := make([]coverageSegment, 0, len(scopeDefs)+1)
		rules := make([]parsedScopeRule, 0, len(scopeDefs))
		for _, def := range scopeDefs {
			segments = append(segments, coverageSegment{
				key:   fmt.Sprintf("scope:%d", def.ID),
				label: def.Definition,
				hosts: make([]coverageSegmentHost, 0),
			})
			rules = append(rules, parseScopeRule(def.Definition))
		}
		unmapped := coverageSegment{
			key:   "scope:unmapped",
			label: "In-scope (unmapped)",
			hosts: make([]coverageSegmentHost, 0),
		}

		for _, host := range hosts {
			matched := false
			for idx, rule := range rules {
				if rule.matches(host.addr) {
					segments[idx].hosts = append(segments[idx].hosts, host)
					matched = true
				}
			}
			if !matched {
				unmapped.hosts = append(unmapped.hosts, host)
			}
		}

		segments = append(segments, unmapped)
		return segments, "scope_rules", nil
	}

	segments := make([]coverageSegment, 0)
	byKey := make(map[string]int)
	for _, host := range hosts {
		cidr := fallbackCIDR24(host.addr)
		if cidr == "" {
			continue
		}
		key := "fallback:" + cidr
		idx, ok := byKey[key]
		if !ok {
			idx = len(segments)
			segments = append(segments, coverageSegment{
				key:   key,
				label: cidr,
				hosts: make([]coverageSegmentHost, 0),
			})
			byKey[key] = idx
		}
		segments[idx].hosts = append(segments[idx].hosts, host)
	}
	return segments, "fallback_24", nil
}

func (db *DB) listCoverageInScopeHosts(projectID int64) ([]coverageSegmentHost, error) {
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

	var hosts []coverageSegmentHost
	for rows.Next() {
		var (
			hostID   int64
			ip       string
			hostname sql.NullString
		)
		if err := rows.Scan(&hostID, &ip, &hostname); err != nil {
			return nil, fmt.Errorf("scan in-scope host: %w", err)
		}
		addr, err := netip.ParseAddr(ip)
		if err != nil || !addr.Is4() {
			continue
		}
		hosts = append(hosts, coverageSegmentHost{
			CoverageMatrixMissingHost: CoverageMatrixMissingHost{
				IPAddress: ip,
				HostID:    hostID,
				Hostname:  strings.TrimSpace(hostname.String),
			},
			addr: addr,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list in-scope hosts rows: %w", err)
	}
	return hosts, nil
}

func (db *DB) loadCoveredHostsByIntent(projectID int64) (map[string]map[string]struct{}, error) {
	intents := CoverageIntentOrder()
	for _, intent := range intents {
		if !ValidIntent(intent) {
			return nil, fmt.Errorf("unsupported intent %q", intent)
		}
	}

	covered := make(map[string]map[string]struct{}, len(intents))
	for _, intent := range intents {
		covered[intent] = make(map[string]struct{})
	}

	placeholders := makePlaceholders(len(intents))
	args := make([]any, 0, len(intents)+1)
	args = append(args, projectID)
	for _, intent := range intents {
		args = append(args, intent)
	}

	rows, err := db.Query(
		fmt.Sprintf(
			`SELECT DISTINCT sii.intent, ho.ip_address
			   FROM host_observation ho
			   JOIN scan_import_intent sii ON sii.scan_import_id = ho.scan_import_id
			  WHERE ho.project_id = ?
			    AND sii.intent IN (%s)`,
			placeholders,
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list covered hosts by intent: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var intent string
		var ip string
		if err := rows.Scan(&intent, &ip); err != nil {
			return nil, fmt.Errorf("scan covered hosts by intent: %w", err)
		}
		intent = strings.ToLower(strings.TrimSpace(intent))
		if _, ok := covered[intent]; !ok {
			continue
		}
		covered[intent][ip] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list covered hosts by intent rows: %w", err)
	}
	return covered, nil
}

func parseScopeRule(definition string) parsedScopeRule {
	def := strings.TrimSpace(definition)
	rule := parsedScopeRule{definition: def}
	if prefix, err := netip.ParsePrefix(def); err == nil {
		rule.prefix = prefix
		rule.isPrefix = true
		return rule
	}
	if addr, err := netip.ParseAddr(def); err == nil {
		rule.addr = addr
		rule.isAddr = true
		return rule
	}
	return rule
}

func fallbackCIDR24(addr netip.Addr) string {
	if !addr.Is4() {
		return ""
	}
	octets := addr.As4()
	return fmt.Sprintf("%d.%d.%d.0/24", octets[0], octets[1], octets[2])
}
