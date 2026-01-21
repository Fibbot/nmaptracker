package db

import (
	"fmt"
	"strings"
)

// HostListItem represents aggregated host list data.
type HostListItem struct {
	ID         int64
	IPAddress  string
	Hostname   string
	InScope    bool
	PortCount  int
	Scanned    int
	Flagged    int
	InProgress int
	Done       int
	ParkingLot int
}

type hostListQuery struct {
	where   string
	having  string
	args    []any
	orderBy string
}

// ListHostsWithSummary returns hosts with aggregated port counts for list view.
func (db *DB) ListHostsWithSummary(projectID int64, inScope *bool, statusFilters []string, sortBy, sortDir string) ([]HostListItem, error) {
	query, err := buildHostListQuery(projectID, inScope, statusFilters, sortBy, sortDir)
	if err != nil {
		return nil, err
	}
	return db.queryHostSummary(query, 0, 0)
}

// ListHostsWithSummaryPaged returns hosts with aggregated port counts and total count.
func (db *DB) ListHostsWithSummaryPaged(projectID int64, inScope *bool, statusFilters []string, sortBy, sortDir string, limit, offset int) ([]HostListItem, int, error) {
	query, err := buildHostListQuery(projectID, inScope, statusFilters, sortBy, sortDir)
	if err != nil {
		return nil, 0, err
	}
	items, err := db.queryHostSummary(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := db.countHostSummary(query)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func buildHostListQuery(projectID int64, inScope *bool, statusFilters []string, sortBy, sortDir string) (hostListQuery, error) {
	var where []string
	var args []any

	where = append(where, "h.project_id = ?")
	args = append(args, projectID)

	if inScope != nil {
		where = append(where, "h.in_scope = ?")
		if *inScope {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}

	orderBy := "h.ip_address"
	switch sortBy {
	case "hostname":
		orderBy = "h.hostname"
	case "ports":
		orderBy = "port_count"
	case "ip", "":
		orderBy = "h.ip_address"
	default:
		return hostListQuery{}, fmt.Errorf("invalid sort column")
	}

	direction := "ASC"
	if strings.EqualFold(sortDir, "desc") {
		direction = "DESC"
	}

	var having string
	if len(statusFilters) > 0 {
		var conditions []string
		for _, status := range statusFilters {
			switch status {
			case "scanned", "flagged", "in_progress", "done", "parking_lot":
				conditions = append(conditions, fmt.Sprintf("SUM(CASE WHEN p.state = 'open' AND p.work_status = '%s' THEN 1 ELSE 0 END) > 0", status))
			}
		}
		if len(conditions) > 0 {
			having = "HAVING (" + strings.Join(conditions, " OR ") + ")"
		}
	}

	return hostListQuery{
		where:   strings.Join(where, " AND "),
		having:  having,
		args:    args,
		orderBy: fmt.Sprintf("%s %s", orderBy, direction),
	}, nil
}

func (db *DB) queryHostSummary(query hostListQuery, limit, offset int) ([]HostListItem, error) {
	sqlQuery := fmt.Sprintf(
		`SELECT h.id,
		        h.ip_address,
		        h.hostname,
		        h.in_scope,
		        COUNT(p.id) AS port_count,
		        COALESCE(SUM(CASE WHEN p.state = 'open' AND p.work_status = 'scanned' THEN 1 ELSE 0 END), 0) AS scanned_count,
		        COALESCE(SUM(CASE WHEN p.state = 'open' AND p.work_status = 'flagged' THEN 1 ELSE 0 END), 0) AS flagged_count,
		        COALESCE(SUM(CASE WHEN p.state = 'open' AND p.work_status = 'in_progress' THEN 1 ELSE 0 END), 0) AS in_progress_count,
		        COALESCE(SUM(CASE WHEN p.state = 'open' AND p.work_status = 'done' THEN 1 ELSE 0 END), 0) AS done_count,
		        COALESCE(SUM(CASE WHEN p.state = 'open' AND p.work_status = 'parking_lot' THEN 1 ELSE 0 END), 0) AS parking_lot_count
		   FROM host h
		   LEFT JOIN port p ON p.host_id = h.id
		  WHERE %s
		  GROUP BY h.id
		  %s
		  ORDER BY %s`,
		query.where,
		query.having,
		query.orderBy,
	)

	if limit > 0 {
		sqlQuery = fmt.Sprintf("%s LIMIT %d OFFSET %d", sqlQuery, limit, offset)
	}

	rows, err := db.Query(sqlQuery, query.args...)
	if err != nil {
		return nil, fmt.Errorf("list hosts with summary: %w", err)
	}
	defer rows.Close()

	var items []HostListItem
	for rows.Next() {
		var item HostListItem
		if err := rows.Scan(
			&item.ID,
			&item.IPAddress,
			&item.Hostname,
			&item.InScope,
			&item.PortCount,
			&item.Scanned,
			&item.Flagged,
			&item.InProgress,
			&item.Done,
			&item.ParkingLot,
		); err != nil {
			return nil, fmt.Errorf("scan host summary: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list host summary rows: %w", err)
	}
	return items, nil
}

func (db *DB) countHostSummary(query hostListQuery) (int, error) {
	sqlQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM (
		    SELECT h.id
		      FROM host h
		      LEFT JOIN port p ON p.host_id = h.id
		     WHERE %s
		     GROUP BY h.id
		     %s
		  )`,
		query.where,
		query.having,
	)
	var total int
	if err := db.QueryRow(sqlQuery, query.args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count host summary: %w", err)
	}
	return total, nil
}
