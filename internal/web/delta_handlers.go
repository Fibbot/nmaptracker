package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sloppy/nmaptracker/internal/db"
)

func (s *Server) apiGetImportDelta(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	query := r.URL.Query()
	baseRaw := strings.TrimSpace(query.Get("base_import_id"))
	targetRaw := strings.TrimSpace(query.Get("target_import_id"))
	if baseRaw == "" || targetRaw == "" {
		s.badRequest(w, fmt.Errorf("base_import_id and target_import_id are required"))
		return
	}

	baseImportID, err := strconv.ParseInt(baseRaw, 10, 64)
	if err != nil {
		s.badRequest(w, fmt.Errorf("invalid base_import_id"))
		return
	}
	targetImportID, err := strconv.ParseInt(targetRaw, 10, 64)
	if err != nil {
		s.badRequest(w, fmt.Errorf("invalid target_import_id"))
		return
	}
	if baseImportID == targetImportID {
		s.badRequest(w, fmt.Errorf("base_import_id and target_import_id must differ"))
		return
	}

	opts := db.DeltaOptions{PreviewSize: 50, IncludeLists: true}
	if raw := strings.TrimSpace(query.Get("preview_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid preview_size"))
			return
		}
		opts.PreviewSize = value
	}
	if raw := strings.TrimSpace(query.Get("include_lists")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			s.badRequest(w, fmt.Errorf("invalid include_lists"))
			return
		}
		opts.IncludeLists = value
	}

	resp, err := s.DB.GetImportDelta(projectID, baseImportID, targetImportID, opts)
	if err != nil {
		if errors.Is(err, db.ErrDeltaImportNotFound) {
			s.errorResponse(w, fmt.Errorf("import not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}

	s.jsonResponse(w, resp, http.StatusOK)
}
