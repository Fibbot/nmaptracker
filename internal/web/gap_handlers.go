package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiGetGaps(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	opts, err := parseGapOptions(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	gaps, err := s.DB.GetGapDashboard(projectID, opts)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, gaps, http.StatusOK)
}

func (s *Server) apiGetMilestoneQueues(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	opts, err := parseGapOptions(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	queues, err := s.DB.GetMilestoneQueues(projectID, opts)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, queues, http.StatusOK)
}

func parseGapOptions(r *http.Request) (db.GapOptions, error) {
	query := r.URL.Query()
	opts := db.GapOptions{
		PreviewSize:  10,
		IncludeLists: true,
	}

	if raw := strings.TrimSpace(query.Get("preview_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return db.GapOptions{}, fmt.Errorf("invalid preview_size")
		}
		opts.PreviewSize = value
	}

	if raw := strings.TrimSpace(query.Get("include_lists")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return db.GapOptions{}, fmt.Errorf("invalid include_lists")
		}
		opts.IncludeLists = value
	}

	return opts, nil
}
