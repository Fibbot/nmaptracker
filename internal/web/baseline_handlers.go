package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiListBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	items, err := s.DB.ListExpectedAssetBaselines(projectID)
	if err != nil {
		s.serverError(w, err)
		return
	}

	type baselineItem struct {
		ID         int64  `json:"id"`
		ProjectID  int64  `json:"project_id"`
		Definition string `json:"definition"`
		Type       string `json:"type"`
		CreatedAt  string `json:"created_at"`
	}

	resp := struct {
		Items []baselineItem `json:"items"`
		Total int            `json:"total"`
	}{
		Items: make([]baselineItem, 0, len(items)),
		Total: len(items),
	}

	for _, item := range items {
		resp.Items = append(resp.Items, baselineItem{
			ID:         item.ID,
			ProjectID:  item.ProjectID,
			Definition: item.Definition,
			Type:       item.Type,
			CreatedAt:  item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	s.jsonResponse(w, resp, http.StatusOK)
}

func (s *Server) apiAddBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	var req struct {
		Definitions []string `json:"definitions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.badRequest(w, err)
		return
	}
	if len(req.Definitions) == 0 {
		s.badRequest(w, fmt.Errorf("definitions are required"))
		return
	}

	added, items, err := s.DB.BulkAddExpectedAssetBaselines(projectID, req.Definitions)
	if err != nil {
		if isBaselineValidationError(err) {
			s.badRequest(w, err)
			return
		}
		s.serverError(w, err)
		return
	}

	type baselineItem struct {
		ID         int64  `json:"id"`
		Definition string `json:"definition"`
		Type       string `json:"type"`
	}
	resp := struct {
		Added int            `json:"added"`
		Items []baselineItem `json:"items"`
	}{
		Added: added,
		Items: make([]baselineItem, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, baselineItem{
			ID:         item.ID,
			Definition: item.Definition,
			Type:       item.Type,
		})
	}

	s.jsonResponse(w, resp, http.StatusCreated)
}

func (s *Server) apiDeleteBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	baselineID, err := strconv.ParseInt(chi.URLParam(r, "baselineID"), 10, 64)
	if err != nil {
		s.badRequest(w, fmt.Errorf("invalid baseline id"))
		return
	}

	if err := s.DB.DeleteExpectedAssetBaseline(projectID, baselineID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.errorResponse(w, fmt.Errorf("baseline not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiEvaluateBaseline(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	result, err := s.DB.EvaluateExpectedAssetBaseline(projectID)
	if err != nil {
		if isBaselineValidationError(err) {
			s.badRequest(w, err)
			return
		}
		s.serverError(w, err)
		return
	}

	resp := struct {
		GeneratedAt string                       `json:"generated_at"`
		ProjectID   int64                        `json:"project_id"`
		Summary     db.BaselineEvaluationSummary `json:"summary"`
		Lists       db.BaselineEvaluationLists   `json:"lists"`
	}{
		GeneratedAt: result.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ProjectID:   result.ProjectID,
		Summary:     result.Summary,
		Lists:       result.Lists,
	}

	s.jsonResponse(w, resp, http.StatusOK)
}

func isBaselineValidationError(err error) bool {
	return errors.Is(err, db.ErrInvalidBaselineDefinition) ||
		errors.Is(err, db.ErrBaselineIPv6Unsupported) ||
		errors.Is(err, db.ErrBaselineCIDRTooBroad)
}
