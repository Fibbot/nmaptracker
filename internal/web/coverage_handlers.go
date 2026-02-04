package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiGetCoverageMatrix(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	includeMissingPreview := true
	if raw := strings.TrimSpace(query.Get("include_missing_preview")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid include_missing_preview"))
			return
		}
		includeMissingPreview = value
	}

	missingPreviewSize := 5
	if raw := strings.TrimSpace(query.Get("missing_preview_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid missing_preview_size"))
			return
		}
		missingPreviewSize = value
	}

	matrix, err := s.DB.GetCoverageMatrix(projectID, db.CoverageMatrixOptions{
		IncludeMissingPreview: includeMissingPreview,
		MissingPreviewSize:    missingPreviewSize,
	})
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.jsonResponse(w, matrix, http.StatusOK)
}

func (s *Server) apiGetCoverageMatrixMissing(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	segmentKey := strings.TrimSpace(query.Get("segment_key"))
	intent := strings.TrimSpace(query.Get("intent"))
	if segmentKey == "" {
		s.badRequest(w, fmt.Errorf("segment_key is required"))
		return
	}
	if intent == "" {
		s.badRequest(w, fmt.Errorf("intent is required"))
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

	items, total, err := s.DB.ListCoverageMatrixMissingHosts(projectID, db.CoverageMatrixMissingOptions{
		SegmentKey: segmentKey,
		Intent:     intent,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		if errors.Is(err, db.ErrCoverageSegmentNotFound) {
			s.errorResponse(w, fmt.Errorf("segment not found"), http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "invalid intent") {
			s.badRequest(w, err)
			return
		}
		s.serverError(w, err)
		return
	}

	resp := struct {
		ProjectID  int64                          `json:"project_id"`
		SegmentKey string                         `json:"segment_key"`
		Intent     string                         `json:"intent"`
		Items      []db.CoverageMatrixMissingHost `json:"items"`
		Total      int                            `json:"total"`
		Page       int                            `json:"page"`
		PageSize   int                            `json:"page_size"`
	}{
		ProjectID:  projectID,
		SegmentKey: segmentKey,
		Intent:     strings.ToLower(intent),
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
	}

	s.jsonResponse(w, resp, http.StatusOK)
}
