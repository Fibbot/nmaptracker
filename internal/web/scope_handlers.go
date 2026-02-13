package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/importer"
	"github.com/sloppy/nmaptracker/internal/scope"
)

func (s *Server) apiListScope(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	rules, err := s.DB.ListScopeDefinitions(id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.jsonResponse(w, rules, http.StatusOK)
}

func (s *Server) apiAddScope(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
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

	count, err := s.DB.BulkAddScopeDefinitions(id, req.Definitions)
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Return updated list
	// Actually plan said: Response: { "added": 3, "rules": [...] }
	rules, err := s.DB.ListScopeDefinitions(id)
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"added": count,
		"rules": rules,
	}, http.StatusCreated)
}

func (s *Server) apiDeleteScope(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}
	scopeID, err := strconv.ParseInt(chi.URLParam(r, "scopeID"), 10, 64)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	if err := s.DB.DeleteScopeDefinitionForProject(projectID, scopeID); err != nil {
		if err == sql.ErrNoRows {
			s.errorResponse(w, fmt.Errorf("scope not found"), http.StatusNotFound)
			return
		}
		s.serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiEvaluateScope(w http.ResponseWriter, r *http.Request) {
	id, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	// Fetch all rules
	rules, err := s.DB.ListScopeDefinitions(id)
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Build Matcher
	var defs []string
	for _, r := range rules {
		defs = append(defs, r.Definition)
	}
	matcher, err := scope.NewMatcher(defs)
	if err != nil {
		s.serverError(w, fmt.Errorf("build matcher: %w", err))
		return
	}

	// Fetch all hosts
	// We need a method to iterate all hosts and update them.
	// Listing all might be heavy. Is there a better way?
	// Maybe an IterateHosts method? Or just ListHosts (MVP).
	// Let's use ListHosts. But existing ListHosts is paged.
	// We need a raw ListAllHosts logic or direct DB update query logic?
	// Doing it in app logic:
	// 1. Get all hosts
	// 2. Check scope
	// 3. Update if changed

	// But ListHostsWithSummaryPaged is complex.
	// We might need a simpler ListHostsForProject db method.
	// Let's assume we can add one or use an existing one?
	// Checking db/host_list.go might reveal it.
	// For now, let's implement a loop over paged results or just add a simple ListAllHosts method to DB?
	// Adding ListAllHosts to DB is cleaner.

	// But wait, there is no ListAllHosts in DB yet.
	// I will write this handler assuming I add ListAllHosts to DB or similar.
	// Or I can just check db directly here? No, Separation of Concerns.
	// I'll add `ListAllHosts(projectID)` to `db` later.

	hosts, err := s.DB.ListHosts(id)
	if err != nil {
		s.serverError(w, err)
		return
	}

	updated := 0
	inScopeCount := 0
	outScopeCount := 0

	for _, h := range hosts {
		inScope := matcher.InScope(h.IPAddress)
		if inScope {
			inScopeCount++
		} else {
			outScopeCount++
		}

		if h.InScope != inScope {
			// Update
			// We need a simple UpdateHostScope method or use UpsertHost (heavy).
			// Let's use UpdateHostScope (to be implemented).
			if err := s.DB.UpdateHostScope(h.ID, inScope); err != nil {
				s.serverError(w, err)
				return
			}
			updated++
		}
	}

	s.jsonResponse(w, map[string]interface{}{
		"updated":      updated,
		"in_scope":     inScopeCount,
		"out_of_scope": outScopeCount,
	}, http.StatusOK)
}

func (s *Server) apiImportXML(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseProjectID(r)
	if err != nil {
		s.badRequest(w, err)
		return
	}

	// Limit upload size to 200MB
	r.Body = http.MaxBytesReader(w, r.Body, 200<<20)
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		s.badRequest(w, fmt.Errorf("parse form: %w", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.badRequest(w, fmt.Errorf("missing file"))
		return
	}
	defer file.Close()

	// Scope
	rules, err := s.DB.ListScopeDefinitions(projectID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	var defs []string
	for _, r := range rules {
		defs = append(defs, r.Definition)
	}
	matcher, err := scope.NewMatcher(defs)
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Import
	manualIntents := collectManualImportIntents(r.MultipartForm.Value["intent"], r.MultipartForm.Value["intents"])
	options := importer.ImportOptions{
		ManualIntents:    manualIntents,
		ScannerLabel:     firstMultipartValue(r.MultipartForm.Value["scanner_label"]),
		ManualSourceIP:   firstMultipartValue(r.MultipartForm.Value["source_ip"]),
		ManualSourcePort: firstMultipartValue(r.MultipartForm.Value["source_port"]),
	}
	if err := importer.ValidateImportOptions(options); err != nil {
		s.badRequest(w, err)
		return
	}
	stats, err := importer.ImportXMLWithOptions(
		s.DB,
		matcher,
		projectID,
		header.Filename,
		file,
		options,
		time.Now().UTC(),
	)
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"success":         true,
		"filename":        header.Filename,
		"hosts_imported":  stats.HostsFound,
		"hosts_skipped":   stats.Skipped,
		"ports_imported":  stats.PortsFound,
		"hosts_in_scope":  stats.InScope,
		"hosts_out_scope": stats.OutScope,
	}, http.StatusOK)
}

func collectManualImportIntents(values ...[]string) []string {
	var intents []string
	for _, group := range values {
		for _, value := range group {
			for _, item := range strings.Split(value, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					intents = append(intents, item)
				}
			}
		}
	}
	return intents
}

func firstMultipartValue(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
