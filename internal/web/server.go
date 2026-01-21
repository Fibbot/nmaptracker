package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sloppy/nmaptracker/internal/db"
)

// Server wires the web handlers and dependencies.
type Server struct {
	DB     *db.DB
	Router chi.Router
}

// NewServer constructs the router and registers routes.
func NewServer(database *db.DB) *Server {
	server := &Server{DB: database}

	r := chi.NewRouter()
	r.Get("/", server.handleRoot)
	r.Get("/projects", server.handleProjectsList)
	r.Get("/projects/{id}", server.handleProjectDashboard)
	r.Get("/projects/{id}/hosts", server.handleProjectHosts)
	r.Get("/projects/{id}/hosts/{hostID}", server.handleHostDetail)
	r.Post("/projects", server.handleProjectsCreate)
	r.Post("/projects/{id}/delete", server.handleProjectsDelete)
	r.Post("/projects/{id}/hosts/{hostID}/notes", server.handleHostNotesUpdate)
	r.Post("/projects/{id}/hosts/{hostID}/ports/{portID}/status", server.handlePortStatusUpdate)
	r.Post("/projects/{id}/hosts/{hostID}/ports/{portID}/notes", server.handlePortNotesUpdate)
	r.Post("/projects/{id}/hosts/{hostID}/bulk-status", server.handleHostBulkStatusUpdate)
	r.Post("/projects/{id}/hosts/bulk-status", server.handleHostListBulkStatusUpdate)
	r.Post("/projects/{id}/ports/bulk-status", server.handlePortNumberBulkStatusUpdate)
	r.Get("/projects/{id}/export", server.handleProjectExport)
	r.Get("/projects/{id}/hosts/{hostID}/export", server.handleHostExport)

	server.Router = r
	return server
}

// Handler exposes the configured router.
func (s *Server) Handler() http.Handler {
	return s.Router
}
