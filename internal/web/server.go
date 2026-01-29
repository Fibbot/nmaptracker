package web

import (
	"embed"
	"io/fs"
	"net/http"

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
		r.Get("/projects", server.apiListProjects)
		r.Post("/projects", server.apiCreateProject)
		r.Get("/projects/{id}", server.apiGetProject)
		r.Delete("/projects/{id}", server.apiDeleteProject)
		r.Get("/projects/{id}/stats", server.apiGetProjectStats)
		r.Get("/projects/{id}/hosts", server.apiListHosts)
		r.Get("/projects/{id}/hosts/{hostID}", server.apiGetHost)
		r.Get("/projects/{id}/hosts/{hostID}/ports", server.apiListPorts)
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
