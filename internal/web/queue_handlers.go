package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiListServiceQueue(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	campaign := strings.ToLower(strings.TrimSpace(query.Get("campaign")))
	if campaign == "" {
		s.badRequest(w, fmt.Errorf("campaign is required"))
		return
	}

	page := 1
	if raw := strings.TrimSpace(query.Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			s.badRequest(w, fmt.Errorf("invalid page"))
			return
		}
		page = value
	}

	pageSize := 50
	if raw := strings.TrimSpace(query.Get("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			s.badRequest(w, fmt.Errorf("invalid page_size"))
			return
		}
		pageSize = value
	}
	if pageSize > 500 {
		pageSize = 500
	}
	offset := (page - 1) * pageSize

	items, total, sourceImportIDs, err := s.DB.ListServiceCampaignQueue(projectID, campaign, pageSize, offset)
	if err != nil {
		if errors.Is(err, db.ErrInvalidServiceCampaign) {
			s.badRequest(w, fmt.Errorf("invalid campaign %q", campaign))
			return
		}
		s.serverError(w, err)
		return
	}

	type statusSummaryResponse struct {
		Scanned    int `json:"scanned"`
		Flagged    int `json:"flagged"`
		InProgress int `json:"in_progress"`
		Done       int `json:"done"`
	}
	type portResponse struct {
		PortID     int64  `json:"port_id"`
		PortNumber int    `json:"port_number"`
		Protocol   string `json:"protocol"`
		State      string `json:"state"`
		Service    string `json:"service"`
		Product    string `json:"product"`
		Version    string `json:"version"`
		WorkStatus string `json:"work_status"`
		LastSeen   string `json:"last_seen"`
	}
	type hostResponse struct {
		HostID        int64                 `json:"host_id"`
		IPAddress     string                `json:"ip_address"`
		Hostname      string                `json:"hostname"`
		LatestSeen    string                `json:"latest_seen"`
		StatusSummary statusSummaryResponse `json:"status_summary"`
		MatchingPorts []portResponse        `json:"matching_ports"`
	}
	type filtersAppliedResponse struct {
		States []string `json:"states"`
	}

	resp := struct {
		GeneratedAt     string                 `json:"generated_at"`
		ProjectID       int64                  `json:"project_id"`
		Campaign        string                 `json:"campaign"`
		FiltersApplied  filtersAppliedResponse `json:"filters_applied"`
		TotalHosts      int                    `json:"total_hosts"`
		Page            int                    `json:"page"`
		PageSize        int                    `json:"page_size"`
		Items           []hostResponse         `json:"items"`
		SourceImportIDs []int64                `json:"source_import_ids"`
	}{
		GeneratedAt: time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z"),
		ProjectID:   projectID,
		Campaign:    campaign,
		FiltersApplied: filtersAppliedResponse{
			States: []string{"open", "open|filtered"},
		},
		TotalHosts:      total,
		Page:            page,
		PageSize:        pageSize,
		Items:           make([]hostResponse, 0, len(items)),
		SourceImportIDs: sourceImportIDs,
	}

	for _, item := range items {
		host := hostResponse{
			HostID:     item.HostID,
			IPAddress:  item.IPAddress,
			Hostname:   item.Hostname,
			LatestSeen: item.LatestSeen.UTC().Format("2006-01-02T15:04:05Z"),
			StatusSummary: statusSummaryResponse{
				Scanned:    item.StatusSummary.Scanned,
				Flagged:    item.StatusSummary.Flagged,
				InProgress: item.StatusSummary.InProgress,
				Done:       item.StatusSummary.Done,
			},
			MatchingPorts: make([]portResponse, 0, len(item.MatchingPorts)),
		}
		for _, port := range item.MatchingPorts {
			host.MatchingPorts = append(host.MatchingPorts, portResponse{
				PortID:     port.PortID,
				PortNumber: port.PortNumber,
				Protocol:   port.Protocol,
				State:      port.State,
				Service:    port.Service,
				Product:    port.Product,
				Version:    port.Version,
				WorkStatus: port.WorkStatus,
				LastSeen:   port.LastSeen.UTC().Format("2006-01-02T15:04:05Z"),
			})
		}
		resp.Items = append(resp.Items, host)
	}

	s.jsonResponse(w, resp, http.StatusOK)
}
