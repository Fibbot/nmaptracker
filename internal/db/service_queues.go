package db

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ServiceCampaignSMB  = "smb"
	ServiceCampaignLDAP = "ldap"
	ServiceCampaignRDP  = "rdp"
	ServiceCampaignHTTP = "http"
)

var ErrInvalidServiceCampaign = errors.New("invalid service campaign")

type serviceCampaignDefinition struct {
	predicate string
}

var serviceCampaignDefinitions = map[string]serviceCampaignDefinition{
	ServiceCampaignSMB: {
		predicate: `(
			(p.port_number = 445 AND lower(p.protocol) = 'tcp')
			OR lower(COALESCE(p.service, '')) LIKE '%smb%'
			OR lower(COALESCE(p.service, '')) LIKE '%microsoft-ds%'
			OR lower(COALESCE(p.service, '')) LIKE '%netbios-ssn%'
		)`,
	},
	ServiceCampaignLDAP: {
		predicate: `(
			((p.port_number = 389 OR p.port_number = 636 OR p.port_number = 3268 OR p.port_number = 3269) AND lower(p.protocol) = 'tcp')
			OR lower(COALESCE(p.service, '')) LIKE '%ldap%'
			OR lower(COALESCE(p.service, '')) LIKE '%ldaps%'
		)`,
	},
	ServiceCampaignRDP: {
		predicate: `(
			(p.port_number = 3389 AND lower(p.protocol) = 'tcp')
			OR lower(COALESCE(p.service, '')) LIKE '%rdp%'
			OR lower(COALESCE(p.service, '')) LIKE '%ms-wbt-server%'
		)`,
	},
	ServiceCampaignHTTP: {
		predicate: `(
			((p.port_number = 80 OR p.port_number = 81 OR p.port_number = 443 OR p.port_number = 8000 OR p.port_number = 8080 OR p.port_number = 8081 OR p.port_number = 8443 OR p.port_number = 8888) AND lower(p.protocol) = 'tcp')
			OR lower(COALESCE(p.service, '')) LIKE '%http%'
			OR lower(COALESCE(p.service, '')) LIKE '%https%'
			OR lower(COALESCE(p.service, '')) LIKE '%http-proxy%'
		)`,
	},
}

// ServiceCampaignPort is one matching port row for a campaign queue host.
type ServiceCampaignPort struct {
	PortID     int64     `json:"port_id"`
	PortNumber int       `json:"port_number"`
	Protocol   string    `json:"protocol"`
	State      string    `json:"state"`
	Service    string    `json:"service"`
	Product    string    `json:"product"`
	Version    string    `json:"version"`
	WorkStatus string    `json:"work_status"`
	LastSeen   time.Time `json:"last_seen"`
}

// ServiceCampaignStatusSummary aggregates work statuses for matching ports.
type ServiceCampaignStatusSummary struct {
	Scanned    int `json:"scanned"`
	Flagged    int `json:"flagged"`
	InProgress int `json:"in_progress"`
	Done       int `json:"done"`
}

// ServiceCampaignHost is one host-level queue item.
type ServiceCampaignHost struct {
	HostID        int64                        `json:"host_id"`
	IPAddress     string                       `json:"ip_address"`
	Hostname      string                       `json:"hostname"`
	MatchingPorts []ServiceCampaignPort        `json:"matching_ports"`
	StatusSummary ServiceCampaignStatusSummary `json:"status_summary"`
	LatestSeen    time.Time                    `json:"latest_seen"`
}

// ListServiceCampaignQueue returns grouped campaign queue hosts with pagination and audit import IDs.
func (db *DB) ListServiceCampaignQueue(projectID int64, campaign string, limit, offset int) ([]ServiceCampaignHost, int, []int64, error) {
	campaign = strings.ToLower(strings.TrimSpace(campaign))
	definition, ok := serviceCampaignDefinitions[campaign]
	if !ok {
		return nil, 0, nil, fmt.Errorf("%w: %q", ErrInvalidServiceCampaign, campaign)
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	baseWhere := fmt.Sprintf(
		`FROM host h
		  JOIN port p ON p.host_id = h.id
		 WHERE h.project_id = ?
		   AND h.in_scope = 1
		   AND p.state IN ('open', 'open|filtered')
		   AND %s`,
		definition.predicate,
	)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (SELECT h.id %s GROUP BY h.id)`, baseWhere)
	var total int
	if err := db.QueryRow(countQuery, projectID).Scan(&total); err != nil {
		return nil, 0, nil, fmt.Errorf("count service queue hosts: %w", err)
	}
	if total == 0 {
		return []ServiceCampaignHost{}, 0, []int64{}, nil
	}

	hostQuery := fmt.Sprintf(
		`SELECT h.id, h.ip_address, h.hostname
		   %s
		  GROUP BY h.id, h.ip_address, h.hostname, h.ip_int
		  ORDER BY CASE WHEN h.ip_int IS NULL THEN 1 ELSE 0 END, h.ip_int, h.ip_address
		  LIMIT ? OFFSET ?`,
		baseWhere,
	)
	hostRows, err := db.Query(hostQuery, projectID, limit, offset)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list service queue hosts: %w", err)
	}
	defer hostRows.Close()

	items := make([]ServiceCampaignHost, 0, limit)
	hostIDs := make([]int64, 0, limit)
	hostIPs := make([]string, 0, limit)
	hostIdx := make(map[int64]int, limit)
	for hostRows.Next() {
		var item ServiceCampaignHost
		if err := hostRows.Scan(&item.HostID, &item.IPAddress, &item.Hostname); err != nil {
			return nil, 0, nil, fmt.Errorf("scan service queue host: %w", err)
		}
		item.Hostname = strings.TrimSpace(item.Hostname)
		item.MatchingPorts = make([]ServiceCampaignPort, 0)
		item.StatusSummary = zeroServiceCampaignStatusSummary()
		items = append(items, item)
		hostIDs = append(hostIDs, item.HostID)
		hostIPs = append(hostIPs, item.IPAddress)
		hostIdx[item.HostID] = len(items) - 1
	}
	if err := hostRows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("list service queue hosts rows: %w", err)
	}
	if len(items) == 0 {
		return []ServiceCampaignHost{}, total, []int64{}, nil
	}

	portArgs := make([]any, 0, len(hostIDs)+1)
	portArgs = append(portArgs, projectID)
	for _, hostID := range hostIDs {
		portArgs = append(portArgs, hostID)
	}
	portQuery := fmt.Sprintf(
		`SELECT h.id, p.id, p.port_number, p.protocol, p.state, p.service, p.product, p.version, p.work_status, p.last_seen
		   FROM host h
		   JOIN port p ON p.host_id = h.id
		  WHERE h.project_id = ?
		    AND h.id IN (%s)
		    AND h.in_scope = 1
		    AND p.state IN ('open', 'open|filtered')
		    AND %s
		  ORDER BY CASE WHEN h.ip_int IS NULL THEN 1 ELSE 0 END, h.ip_int, h.ip_address, p.port_number, p.protocol`,
		makePlaceholders(len(hostIDs)),
		definition.predicate,
	)
	portRows, err := db.Query(portQuery, portArgs...)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list service queue ports: %w", err)
	}
	defer portRows.Close()

	for portRows.Next() {
		var hostID int64
		var port ServiceCampaignPort
		if err := portRows.Scan(
			&hostID,
			&port.PortID,
			&port.PortNumber,
			&port.Protocol,
			&port.State,
			&port.Service,
			&port.Product,
			&port.Version,
			&port.WorkStatus,
			&port.LastSeen,
		); err != nil {
			return nil, 0, nil, fmt.Errorf("scan service queue port: %w", err)
		}

		idx, ok := hostIdx[hostID]
		if !ok {
			continue
		}
		item := &items[idx]
		port.Protocol = strings.ToLower(strings.TrimSpace(port.Protocol))
		port.State = strings.TrimSpace(port.State)
		port.Service = strings.TrimSpace(port.Service)
		port.Product = strings.TrimSpace(port.Product)
		port.Version = strings.TrimSpace(port.Version)
		port.WorkStatus = strings.ToLower(strings.TrimSpace(port.WorkStatus))
		item.MatchingPorts = append(item.MatchingPorts, port)
		accumulateServiceCampaignStatus(&item.StatusSummary, port.WorkStatus)
		if item.LatestSeen.IsZero() || port.LastSeen.After(item.LatestSeen) {
			item.LatestSeen = port.LastSeen
		}
	}
	if err := portRows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("list service queue ports rows: %w", err)
	}

	sourceImportIDs, err := db.listServiceQueueSourceImportIDs(projectID, hostIPs)
	if err != nil {
		return nil, 0, nil, err
	}

	return items, total, sourceImportIDs, nil
}

func zeroServiceCampaignStatusSummary() ServiceCampaignStatusSummary {
	return ServiceCampaignStatusSummary{
		Scanned:    0,
		Flagged:    0,
		InProgress: 0,
		Done:       0,
	}
}

func accumulateServiceCampaignStatus(summary *ServiceCampaignStatusSummary, status string) {
	switch status {
	case "scanned":
		summary.Scanned++
	case "flagged":
		summary.Flagged++
	case "in_progress":
		summary.InProgress++
	case "done":
		summary.Done++
	}
}

func (db *DB) listServiceQueueSourceImportIDs(projectID int64, hostIPs []string) ([]int64, error) {
	if len(hostIPs) == 0 {
		return []int64{}, nil
	}

	args := make([]any, 0, len(hostIPs)+1)
	args = append(args, projectID)
	for _, ip := range hostIPs {
		args = append(args, ip)
	}

	rows, err := db.Query(
		fmt.Sprintf(
			`SELECT DISTINCT scan_import_id
			   FROM host_observation
			  WHERE project_id = ?
			    AND ip_address IN (%s)
			  ORDER BY scan_import_id`,
			makePlaceholders(len(hostIPs)),
		),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list service queue source import ids: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan service queue source import id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list service queue source import ids rows: %w", err)
	}
	return ids, nil
}
