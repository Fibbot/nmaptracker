package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"
)

var (
	// ErrInvalidBaselineDefinition is returned when a baseline definition is not valid IPv4 IP/CIDR syntax.
	ErrInvalidBaselineDefinition = errors.New("invalid baseline definition")
	// ErrBaselineIPv6Unsupported is returned when callers submit IPv6 baseline definitions.
	ErrBaselineIPv6Unsupported = errors.New("ipv6 baseline definitions are not supported")
	// ErrBaselineCIDRTooBroad is returned when a CIDR is broader than /16.
	ErrBaselineCIDRTooBroad = errors.New("cidr broader than /16 is not allowed")
)

// BaselineSeenHost represents an observed host that is outside the expected baseline.
type BaselineSeenHost struct {
	HostID    int64  `json:"host_id"`
	IPAddress string `json:"ip_address"`
	Hostname  string `json:"hostname"`
	InScope   bool   `json:"in_scope"`
}

// BaselineEvaluationSummary contains aggregate expected-vs-observed counts.
type BaselineEvaluationSummary struct {
	ExpectedTotal                        int `json:"expected_total"`
	ObservedTotal                        int `json:"observed_total"`
	ExpectedButUnseen                    int `json:"expected_but_unseen"`
	SeenButOutOfScope                    int `json:"seen_but_out_of_scope"`
	SeenButOutOfScopeAndMarkedInScope    int `json:"seen_but_out_of_scope_and_marked_in_scope"`
	SeenButOutOfScopeAndMarkedOutOfScope int `json:"seen_but_out_of_scope_and_marked_out_scope"`
}

// BaselineEvaluationLists contains the baseline evaluation result lists.
type BaselineEvaluationLists struct {
	ExpectedButUnseen                    []string           `json:"expected_but_unseen"`
	SeenButOutOfScope                    []BaselineSeenHost `json:"seen_but_out_of_scope"`
	SeenButOutOfScopeAndMarkedInScope    []BaselineSeenHost `json:"seen_but_out_of_scope_and_marked_in_scope"`
	SeenButOutOfScopeAndMarkedOutOfScope []BaselineSeenHost `json:"seen_but_out_of_scope_and_marked_out_scope"`
}

// BaselineEvaluation captures expected baseline evaluation output for one project.
type BaselineEvaluation struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	ProjectID   int64                     `json:"project_id"`
	Summary     BaselineEvaluationSummary `json:"summary"`
	Lists       BaselineEvaluationLists   `json:"lists"`
}

// ListExpectedAssetBaselines returns baselines for a project.
func (db *DB) ListExpectedAssetBaselines(projectID int64) ([]ExpectedAssetBaseline, error) {
	rows, err := db.Query(
		`SELECT id, project_id, definition, type, created_at
		   FROM expected_asset_baseline
		  WHERE project_id = ?
		  ORDER BY id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list expected asset baselines: %w", err)
	}
	defer rows.Close()

	var items []ExpectedAssetBaseline
	for rows.Next() {
		var item ExpectedAssetBaseline
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Definition, &item.Type, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan expected asset baseline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expected asset baselines rows: %w", err)
	}
	return items, nil
}

// BulkAddExpectedAssetBaselines inserts baseline definitions for a project.
func (db *DB) BulkAddExpectedAssetBaselines(projectID int64, defs []string) (int, []ExpectedAssetBaseline, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO expected_asset_baseline (project_id, definition, type) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, nil, fmt.Errorf("prepare baseline insert: %w", err)
	}
	defer stmt.Close()

	added := 0
	addedDefs := make([]string, 0, len(defs))
	seenDefs := make(map[string]struct{}, len(defs))

	for _, raw := range defs {
		def, typ, err := normalizeBaselineDefinition(raw)
		if err != nil {
			return added, nil, err
		}
		if def == "" {
			continue
		}
		if _, exists := seenDefs[def]; exists {
			continue
		}
		seenDefs[def] = struct{}{}

		res, err := stmt.Exec(projectID, def, typ)
		if err != nil {
			return added, nil, fmt.Errorf("insert expected asset baseline: %w", err)
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			added++
			addedDefs = append(addedDefs, def)
		}
	}

	if err := tx.Commit(); err != nil {
		return added, nil, fmt.Errorf("commit baseline insert: %w", err)
	}

	if len(addedDefs) == 0 {
		return 0, []ExpectedAssetBaseline{}, nil
	}
	items, err := db.listExpectedAssetBaselinesByDefinitions(projectID, addedDefs)
	if err != nil {
		return added, nil, err
	}
	return added, items, nil
}

// DeleteExpectedAssetBaseline removes one baseline, scoped by project.
func (db *DB) DeleteExpectedAssetBaseline(projectID, baselineID int64) error {
	res, err := db.Exec(`DELETE FROM expected_asset_baseline WHERE id = ? AND project_id = ?`, baselineID, projectID)
	if err != nil {
		return fmt.Errorf("delete expected asset baseline: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// EvaluateExpectedAssetBaseline evaluates expected-vs-observed drift for a project.
func (db *DB) EvaluateExpectedAssetBaseline(projectID int64) (BaselineEvaluation, error) {
	baselines, err := db.ListExpectedAssetBaselines(projectID)
	if err != nil {
		return BaselineEvaluation{}, err
	}
	hosts, err := db.listBaselineObservedHosts(projectID)
	if err != nil {
		return BaselineEvaluation{}, err
	}

	expected := make(map[uint32]struct{})
	for _, baseline := range baselines {
		definition, typ, err := normalizeBaselineDefinition(baseline.Definition)
		if err != nil {
			return BaselineEvaluation{}, fmt.Errorf("invalid stored baseline definition %q: %w", baseline.Definition, err)
		}
		if definition == "" {
			continue
		}
		if typ == "ip" {
			addr, _ := netip.ParseAddr(definition)
			expected[ipToUint32(addr)] = struct{}{}
			continue
		}

		prefix, _ := netip.ParsePrefix(definition)
		base := ipToUint32(prefix.Addr())
		hostCount := uint32(1) << uint32(32-prefix.Bits())
		for i := uint32(0); i < hostCount; i++ {
			expected[base+i] = struct{}{}
		}
	}

	observed := make(map[uint32]struct{}, len(hosts))
	seenButOut := make([]BaselineSeenHost, 0)
	seenButOutMarkedInScope := make([]BaselineSeenHost, 0)
	seenButOutMarkedOutScope := make([]BaselineSeenHost, 0)

	for _, host := range hosts {
		ipInt, ok := parseIPv4String(host.IPAddress)
		if !ok {
			continue
		}
		observed[ipInt] = struct{}{}

		if _, expectedHost := expected[ipInt]; expectedHost {
			continue
		}

		seenButOut = append(seenButOut, host)
		if host.InScope {
			seenButOutMarkedInScope = append(seenButOutMarkedInScope, host)
		} else {
			seenButOutMarkedOutScope = append(seenButOutMarkedOutScope, host)
		}
	}

	expectedButUnseen := make([]string, 0)
	for ipInt := range expected {
		if _, found := observed[ipInt]; !found {
			expectedButUnseen = append(expectedButUnseen, uint32ToIPv4(ipInt))
		}
	}
	sort.Slice(expectedButUnseen, func(i, j int) bool {
		left, _ := parseIPv4String(expectedButUnseen[i])
		right, _ := parseIPv4String(expectedButUnseen[j])
		return left < right
	})

	byIP := func(items []BaselineSeenHost) {
		sort.Slice(items, func(i, j int) bool {
			left, okLeft := parseIPv4String(items[i].IPAddress)
			right, okRight := parseIPv4String(items[j].IPAddress)
			if okLeft != okRight {
				return okLeft
			}
			if okLeft && okRight && left != right {
				return left < right
			}
			return items[i].IPAddress < items[j].IPAddress
		})
	}
	byIP(seenButOut)
	byIP(seenButOutMarkedInScope)
	byIP(seenButOutMarkedOutScope)

	return BaselineEvaluation{
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		ProjectID:   projectID,
		Summary: BaselineEvaluationSummary{
			ExpectedTotal:                        len(expected),
			ObservedTotal:                        len(observed),
			ExpectedButUnseen:                    len(expectedButUnseen),
			SeenButOutOfScope:                    len(seenButOut),
			SeenButOutOfScopeAndMarkedInScope:    len(seenButOutMarkedInScope),
			SeenButOutOfScopeAndMarkedOutOfScope: len(seenButOutMarkedOutScope),
		},
		Lists: BaselineEvaluationLists{
			ExpectedButUnseen:                    expectedButUnseen,
			SeenButOutOfScope:                    seenButOut,
			SeenButOutOfScopeAndMarkedInScope:    seenButOutMarkedInScope,
			SeenButOutOfScopeAndMarkedOutOfScope: seenButOutMarkedOutScope,
		},
	}, nil
}

func (db *DB) listExpectedAssetBaselinesByDefinitions(projectID int64, definitions []string) ([]ExpectedAssetBaseline, error) {
	if len(definitions) == 0 {
		return []ExpectedAssetBaseline{}, nil
	}

	args := make([]any, 0, len(definitions)+1)
	args = append(args, projectID)
	for _, definition := range definitions {
		args = append(args, definition)
	}

	rows, err := db.Query(
		fmt.Sprintf(
			`SELECT id, project_id, definition, type, created_at
			   FROM expected_asset_baseline
			  WHERE project_id = ?
			    AND definition IN (%s)
			  ORDER BY id`,
			makePlaceholders(len(definitions)),
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list inserted expected asset baselines: %w", err)
	}
	defer rows.Close()

	items := make([]ExpectedAssetBaseline, 0, len(definitions))
	for rows.Next() {
		var item ExpectedAssetBaseline
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Definition, &item.Type, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan inserted expected asset baseline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inserted expected asset baselines: %w", err)
	}
	return items, nil
}

func (db *DB) listBaselineObservedHosts(projectID int64) ([]BaselineSeenHost, error) {
	rows, err := db.Query(
		`SELECT id, ip_address, hostname, in_scope
		   FROM host
		  WHERE project_id = ?
		  ORDER BY CASE WHEN ip_int IS NULL THEN 1 ELSE 0 END, ip_int, ip_address`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list baseline observed hosts: %w", err)
	}
	defer rows.Close()

	hosts := make([]BaselineSeenHost, 0)
	for rows.Next() {
		var host BaselineSeenHost
		if err := rows.Scan(&host.HostID, &host.IPAddress, &host.Hostname, &host.InScope); err != nil {
			return nil, fmt.Errorf("scan baseline observed host: %w", err)
		}
		host.Hostname = strings.TrimSpace(host.Hostname)
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate baseline observed hosts: %w", err)
	}
	return hosts, nil
}

func normalizeBaselineDefinition(raw string) (string, string, error) {
	definition := strings.TrimSpace(raw)
	if definition == "" {
		return "", "", nil
	}

	if addr, err := netip.ParseAddr(definition); err == nil {
		if !addr.Is4() {
			return "", "", fmt.Errorf("%w: %q", ErrBaselineIPv6Unsupported, raw)
		}
		return addr.String(), "ip", nil
	}

	prefix, err := netip.ParsePrefix(definition)
	if err != nil {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidBaselineDefinition, raw)
	}
	if !prefix.Addr().Is4() {
		return "", "", fmt.Errorf("%w: %q", ErrBaselineIPv6Unsupported, raw)
	}
	if prefix.Bits() < 16 {
		return "", "", fmt.Errorf("%w: %q", ErrBaselineCIDRTooBroad, raw)
	}
	return prefix.Masked().String(), "cidr", nil
}

func parseIPv4String(raw string) (uint32, bool) {
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.Is4() {
		return 0, false
	}
	return ipToUint32(addr), true
}

func ipToUint32(addr netip.Addr) uint32 {
	octets := addr.As4()
	return uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
}

func uint32ToIPv4(value uint32) string {
	addr := netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
	return addr.String()
}
