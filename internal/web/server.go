package web

import (
	"embed"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/db"
)

//go:embed frontend/*
var frontendFS embed.FS

// Server wires the web handlers and dependencies.
type Server struct {
	DB     *db.DB
	Router chi.Router
}

// NewServer constructs the router and registers routes.
func NewServer(database *db.DB) *Server {
	server := &Server{DB: database}

	r := chi.NewRouter()

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Use(csrfGuard)
		r.Get("/projects", server.apiListProjects)
		r.Post("/projects", server.apiCreateProject)
		r.Get("/projects/{id}", server.apiGetProject)
		r.Delete("/projects/{id}", server.apiDeleteProject)
		r.Put("/projects/{id}", server.apiUpdateProject)
		r.Get("/projects/{id}/stats", server.apiGetProjectStats)
		r.Get("/projects/{id}/hosts", server.apiListHosts)
		r.Get("/projects/{id}/ports/all", server.apiListProjectPorts)
		r.Post("/projects/{id}/ports/bulk-status", server.apiProjectBulkPortStatus)
		r.Get("/projects/{id}/hosts/{hostID}", server.apiGetHost)
		r.Get("/projects/{id}/hosts/{hostID}/ports", server.apiListPorts)
		r.Delete("/projects/{id}/hosts/{hostID}", server.apiDeleteHost)
		r.Put("/projects/{id}/hosts/{hostID}/notes", server.apiUpdateHostNotes)
		r.Put("/projects/{id}/hosts/{hostID}/ports/{portID}/status", server.apiUpdatePortStatus)
		r.Put("/projects/{id}/hosts/{hostID}/ports/{portID}/notes", server.apiUpdatePortNotes)
		r.Post("/projects/{id}/hosts/{hostID}/bulk-status", server.apiHostBulkStatus)

		// Scope
		r.Get("/projects/{id}/scope", server.apiListScope)
		r.Post("/projects/{id}/scope", server.apiAddScope)
		r.Delete("/projects/{id}/scope/{scopeID}", server.apiDeleteScope)
		r.Post("/projects/{id}/scope/evaluate", server.apiEvaluateScope)

		// Import
		r.Post("/projects/{id}/import", server.apiImportXML)
		r.Get("/projects/{id}/imports", server.apiListImports)
		r.Put("/projects/{id}/imports/{importID}/intents", server.apiSetImportIntents)
		r.Get("/projects/{id}/coverage-matrix", server.apiGetCoverageMatrix)
		r.Get("/projects/{id}/coverage-matrix/missing", server.apiGetCoverageMatrixMissing)
		r.Get("/projects/{id}/gaps", server.apiGetGaps)
		r.Get("/projects/{id}/queues/milestones", server.apiGetMilestoneQueues)

		// Exports (files)
		r.Get("/projects/{id}/export", server.handleProjectExport)
		r.Get("/projects/{id}/hosts/{hostID}/export", server.handleHostExport)
	})

	// Static Files
	frontendContent, _ := fs.Sub(frontendFS, "frontend")
	fileServer := http.FileServer(http.FS(frontendContent))
	r.Handle("/*", fileServer)

	server.Router = r
	return server
}

// Handler exposes the configured router.
func (s *Server) Handler() http.Handler {
	return s.Router
}

func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if origin == "null" {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		originURL, err := url.Parse(origin)
		if err != nil || originURL.Host == "" {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}

		originHost := originURL.Hostname()
		if !isLocalHost(originHost) {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}

		requestHost := r.Host
		requestHostName := requestHost
		requestPort := ""
		if strings.Contains(requestHost, ":") {
			host, port, err := net.SplitHostPort(requestHost)
			if err == nil {
				requestHostName = host
				requestPort = port
			}
		}
		if !isLocalHost(requestHostName) {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}

		if originURL.Port() != "" && requestPort != "" && originURL.Port() != requestPort {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1":
		return true
	default:
		return false
	}
}
