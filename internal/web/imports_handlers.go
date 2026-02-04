package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiListImports(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	items, err := s.DB.ListScanImportsWithIntents(projectID)
	if err != nil {
		s.serverError(w, err)
		return
	}

	type intentResponse struct {
		Intent     string  `json:"intent"`
		Source     string  `json:"source"`
		Confidence float64 `json:"confidence"`
	}
	type importResponse struct {
		ID         int64            `json:"id"`
		ProjectID  int64            `json:"project_id"`
		Filename   string           `json:"filename"`
		ImportTime string           `json:"import_time"`
		HostsFound int              `json:"hosts_found"`
		PortsFound int              `json:"ports_found"`
		Intents    []intentResponse `json:"intents"`
	}

	resp := struct {
		Items []importResponse `json:"items"`
		Total int              `json:"total"`
	}{
		Items: make([]importResponse, 0, len(items)),
		Total: len(items),
	}

	for _, item := range items {
		mapped := importResponse{
			ID:         item.ID,
			ProjectID:  item.ProjectID,
			Filename:   item.Filename,
			ImportTime: item.ImportTime.UTC().Format("2006-01-02T15:04:05Z"),
			HostsFound: item.HostsFound,
			PortsFound: item.PortsFound,
			Intents:    make([]intentResponse, 0, len(item.Intents)),
		}
		for _, intent := range item.Intents {
			mapped.Intents = append(mapped.Intents, intentResponse{
				Intent:     intent.Intent,
				Source:     intent.Source,
				Confidence: intent.Confidence,
			})
		}
		resp.Items = append(resp.Items, mapped)
	}

	s.jsonResponse(w, resp, http.StatusOK)
}

func (s *Server) apiSetImportIntents(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	importID, err := strconv.ParseInt(chi.URLParam(r, "importID"), 10, 64)
	if err != nil {
		s.badRequest(w, fmt.Errorf("invalid import id"))
		return
	}

	var req struct {
		Intents []db.ScanImportIntentInput `json:"intents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}

	for _, intent := range req.Intents {
		if !db.ValidIntent(intent.Intent) {
			s.badRequest(w, fmt.Errorf("invalid intent %q", intent.Intent))
			return
		}
		if !db.ValidIntentSource(intent.Source) {
			s.badRequest(w, fmt.Errorf("invalid source %q", intent.Source))
			return
		}
		if intent.Confidence < 0 || intent.Confidence > 1 {
			s.badRequest(w, fmt.Errorf("invalid confidence"))
			return
		}
	}

	if err := s.DB.SetScanImportIntents(projectID, importID, req.Intents); err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows") {
			s.errorResponse(w, fmt.Errorf("import not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}
