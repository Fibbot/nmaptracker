package db

import "fmt"

// WorkStatusCounts holds counts of open ports by workflow status.
type WorkStatusCounts struct {
	Scanned    int
	Flagged    int
	InProgress int
	Done       int
	ParkingLot int
}

// DashboardStats summarizes project-level counts for the dashboard.
type DashboardStats struct {
	TotalHosts    int
	InScopeHosts  int
	OutScopeHosts int
	WorkStatus    WorkStatusCounts
}

// GetDashboardStats returns host and open-port status counts for a project.
func (db *DB) GetDashboardStats(projectID int64) (DashboardStats, error) {
	var stats DashboardStats
	if err := db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN in_scope = 1 THEN 1 ELSE 0 END), 0)
		   FROM host WHERE project_id = ?`,
		projectID,
	).Scan(&stats.TotalHosts, &stats.InScopeHosts); err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard host counts: %w", err)
	}
	stats.OutScopeHosts = stats.TotalHosts - stats.InScopeHosts

	rows, err := db.Query(
		`SELECT p.work_status, COUNT(*)
		   FROM port p
		   JOIN host h ON h.id = p.host_id
		  WHERE h.project_id = ? AND p.state = 'open'
		  GROUP BY p.work_status`,
		projectID,
	)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard port counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return DashboardStats{}, fmt.Errorf("scan dashboard port counts: %w", err)
		}
		switch status {
		case "scanned":
			stats.WorkStatus.Scanned = count
		case "flagged":
			stats.WorkStatus.Flagged = count
		case "in_progress":
			stats.WorkStatus.InProgress = count
		case "done":
			stats.WorkStatus.Done = count
		case "parking_lot":
			stats.WorkStatus.ParkingLot = count
		}
	}
	if err := rows.Err(); err != nil {
		return DashboardStats{}, fmt.Errorf("dashboard port counts: %w", err)
	}

	return stats, nil
}
