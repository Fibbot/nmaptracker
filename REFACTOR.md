# NmapTracker Frontend Refactor: Templ → Vanilla HTML/JS/CSS

## Objective

Rip out the templ-based frontend and replace it with vanilla HTML/JS/CSS served via `go:embed`. The goal is a simple, functional MVP - not pretty, just working. Backend logic stays intact; we're only changing how the UI is rendered and served.

## Current Architecture

```
cmd/nmap-tracker/main.go    - CLI entry point (serve, import, export, projects commands)
internal/
  db/                       - SQLite database layer (KEEP AS-IS)
    models.go               - Project, Host, Port, ScanImport, ScopeDefinition structs
    dashboard.go            - DashboardStats, WorkStatusCounts aggregation
    host.go, port.go, etc.  - CRUD operations
  importer/                 - Nmap XML parsing (KEEP AS-IS)
  export/                   - JSON/CSV export (KEEP AS-IS)
  scope/                    - Scope matching logic (KEEP AS-IS)
  web/                      - HTTP handlers + templ rendering (REFACTOR THIS)
    server.go               - Chi router setup
    handlers.go             - Request handlers
    *.templ, *_templ.go     - DELETE THESE
    render.go               - DELETE THIS
    filters.go              - Keep filter parsing logic
    port_helpers.go         - Keep helper functions
    host_list_links.go      - Keep URL building helpers
```

## Target Architecture

```
cmd/nmap-tracker/main.go    - No changes needed
internal/
  db/                       - No changes
  importer/                 - No changes
  export/                   - No changes  
  scope/                    - No changes
  web/
    server.go               - Update to serve static files + API routes
    handlers.go             - Convert to JSON API handlers
    api.go                  - New file for JSON response helpers
    filters.go              - Keep
    port_helpers.go         - Keep  
    host_list_links.go      - Keep or adapt for API
frontend/                   - NEW: embedded static files
  index.html                - Projects list (landing page)
  project.html              - Project dashboard
  hosts.html                - Hosts list with filters
  host.html                 - Host detail with ports
  css/
    style.css               - Basic functional styling
  js/
    app.js                  - Shared utilities, fetch wrappers
    projects.js             - Projects page logic
    dashboard.js            - Dashboard page logic
    hosts.js                - Hosts list logic
    host.js                 - Host detail logic
```

## Data Models (from internal/db/models.go)

```go
type Project struct {
    ID        int64
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Host struct {
    ID        int64
    ProjectID int64
    IPAddress string
    Hostname  string
    OSGuess   string
    InScope   bool
    Notes     string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Port struct {
    ID           int64
    HostID       int64
    PortNumber   int
    Protocol     string
    State        string      // open, closed, filtered
    Service      string
    Version      string
    Product      string
    ExtraInfo    string
    WorkStatus   string      // scanned, flagged, in_progress, done, parking_lot
    ScriptOutput string
    Notes        string
    LastSeen     time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type DashboardStats struct {
    TotalHosts    int
    InScopeHosts  int
    OutScopeHosts int
    WorkStatus    WorkStatusCounts
}

type WorkStatusCounts struct {
    Scanned    int
    Flagged    int
    InProgress int
    Done       int
    ParkingLot int
}

// Used in hosts list
type HostListItem struct {
    ID         int64
    IPAddress  string
    Hostname   string
    InScope    bool
    PortCount  int
    Scanned    int
    Flagged    int
    InProgress int
    Done       int
    ParkingLot int
}
```

## API Endpoints to Implement

Replace the current HTML-rendering handlers with JSON API endpoints. Keep the same URL patterns but under `/api/` prefix.

### Projects
```
GET  /api/projects                    → []Project
POST /api/projects                    → Project (body: {"name": "..."})
DELETE /api/projects/{id}             → 204 No Content
```

### Project Dashboard
```
GET  /api/projects/{id}               → Project
GET  /api/projects/{id}/stats         → DashboardStats
```

### Hosts
```
GET  /api/projects/{id}/hosts         → {items: []HostListItem, total: int}
     Query params: subnet, status, in_scope, sort, dir, page, page_size
```

### Host Detail
```
GET  /api/projects/{id}/hosts/{hostID}           → Host
GET  /api/projects/{id}/hosts/{hostID}/ports     → []Port
     Query params: state (open, closed, filtered - can be multiple)
PUT  /api/projects/{id}/hosts/{hostID}/notes     → Host
     Body: {"notes": "..."}
```

### Port Updates
```
PUT  /api/projects/{id}/hosts/{hostID}/ports/{portID}/status  → Port
     Body: {"status": "flagged"}
PUT  /api/projects/{id}/hosts/{hostID}/ports/{portID}/notes   → Port
     Body: {"notes": "..."}
```

### Bulk Operations (SIMPLIFY FOR MVP)
For MVP, we can skip the bulk operations or implement them as simple endpoints:
```
POST /api/projects/{id}/hosts/{hostID}/bulk-status  → 204
     Body: {"status": "done"}
     (Sets all open ports on host to status)
```

### Exports (Keep existing - they already return files)
```
GET /projects/{id}/export?format=json|csv
GET /projects/{id}/hosts/{hostID}/export?format=json|csv
```

## Static File Serving with go:embed

In `internal/web/server.go`:

```go
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

type Server struct {
    DB     *db.DB
    Router chi.Router
}

func NewServer(database *db.DB) *Server {
    server := &Server{DB: database}
    r := chi.NewRouter()
    
    // API routes
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
    })
    
    // Export routes (keep as-is, they return files)
    r.Get("/projects/{id}/export", server.handleProjectExport)
    r.Get("/projects/{id}/hosts/{hostID}/export", server.handleHostExport)
    
    // Static files - serve frontend
    frontendContent, _ := fs.Sub(frontendFS, "frontend")
    fileServer := http.FileServer(http.FS(frontendContent))
    r.Handle("/*", fileServer)
    
    server.Router = r
    return server
}
```

**IMPORTANT**: The `frontend/` directory must be inside `internal/web/` for the embed to work, since the embed directive is relative to the Go file's location. So the actual path will be `internal/web/frontend/`.

## Frontend Pages

### index.html - Projects List
- List all projects with name and created date
- Form to create new project
- Delete button per project (with confirmation)
- Links to project dashboard

### project.html - Project Dashboard  
- Show project name
- Display stats: total hosts, in-scope, out-of-scope
- Display workflow counts: scanned, flagged, in_progress, done, parking_lot
- Calculate and show progress percentage
- Links to hosts list and exports

### hosts.html - Hosts List
- Filter form: subnet (text), status (select), in_scope (select), sort (select), direction (select)
- Table: IP (link to detail), hostname, port count, status summary, in_scope
- Pagination: previous/next links, "Page X of Y"
- Keep it simple - full page reload on filter/pagination for MVP

### host.html - Host Detail
- Show IP, hostname, OS guess, scope status
- Host notes textarea with save button
- Port state filter checkboxes (open/closed/filtered)
- Ports table: port/proto, state, service, status dropdown, notes textarea
- Each port row has its own save buttons for status and notes
- Show script output in expandable section if present

## JavaScript Approach

### app.js - Shared Utilities
```javascript
// Base API URL
const API = '/api';

// Fetch wrapper with error handling
async function api(path, options = {}) {
    const res = await fetch(API + path, {
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
    }
    if (res.status === 204) return null;
    return res.json();
}

// Get URL params
function getParam(name) {
    return new URLSearchParams(window.location.search).get(name);
}

// Get path segment (e.g., /project.html?id=5 → 5)
function getProjectId() {
    return getParam('id');
}

function getHostId() {
    return getParam('hostId');
}
```

### Page-Specific JS
Each page loads on DOMContentLoaded, fetches its data, and renders. Keep it simple:
- Fetch data
- Build HTML strings or use template literals
- Insert into DOM
- Attach event listeners

No framework, no virtual DOM, no reactivity - just imperative DOM manipulation.

## CSS Approach

Minimal functional styling. No need for beauty, just usability:
- Basic reset
- Simple table styling with borders
- Form elements that are usable
- Some spacing/padding
- Maybe a max-width container

```css
* { box-sizing: border-box; }
body { font-family: system-ui, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
table { width: 100%; border-collapse: collapse; }
th, td { border: 1px solid #ccc; padding: 8px; text-align: left; }
input, select, textarea, button { padding: 8px; margin: 4px 0; }
button { cursor: pointer; }
.error { color: red; }
nav a { margin-right: 16px; }
```

## Files to Delete

Remove all templ-related files from `internal/web/`:
- `base_layout.templ`
- `base_layout_templ.go`
- `dashboard.templ`
- `dashboard_templ.go`
- `hosts.templ`
- `hosts_templ.go`
- `host_detail.templ`
- `host_detail_templ.go`
- `projects.templ`
- `projects_templ.go`
- `render.go`
- `templ_generate.go`

## Dependencies to Remove

In `go.mod`, remove:
```
github.com/a-h/templ
```

Run `go mod tidy` after refactoring.

## Migration Checklist

1. [ ] Create `internal/web/frontend/` directory structure
2. [ ] Create basic HTML files (index.html, project.html, hosts.html, host.html)
3. [ ] Create `css/style.css` with minimal styling
4. [ ] Create `js/app.js` with shared utilities
5. [ ] Create page-specific JS files
6. [ ] Update `internal/web/server.go` with embed directive and new routes
7. [ ] Create `internal/web/api.go` with JSON response helpers
8. [ ] Convert handlers in `handlers.go` to return JSON (or create new api handlers)
9. [ ] Delete all `*.templ` and `*_templ.go` files
10. [ ] Delete `render.go` and `templ_generate.go`
11. [ ] Remove templ from go.mod
12. [ ] Run `go mod tidy`
13. [ ] Test: `go build ./cmd/nmap-tracker && ./nmap-tracker serve`
14. [ ] Verify all pages load and function

## Simplifications for MVP

1. **No HTMX partial updates** - Full page navigation is fine
2. **No inline editing** - Navigate to detail pages, use forms with submit buttons
3. **Skip bulk operations on hosts list** - Just do per-host bulk on host detail page
4. **Skip subnet filtering** - Can add later if needed
5. **Basic pagination** - Just prev/next, no page size selector
6. **No fancy UI** - Tables, forms, links. That's it.

## Testing the Result

After refactoring:
```bash
# Build
go build ./cmd/nmap-tracker

# Create a test project
./nmap-tracker projects create "Test Project"

# Import sample data
./nmap-tracker import --project "Test Project" sampleNmap.xml

# Start server
./nmap-tracker serve

# Open browser to http://localhost:8080
```

Verify:
- Projects list shows "Test Project"
- Clicking project shows dashboard with stats
- Hosts page shows imported hosts
- Host detail shows ports
- Can update port status and notes
- Exports download correctly

## Notes for the Agent

- The existing `internal/db/` layer is solid - don't modify it
- Existing export handlers can stay mostly as-is
- Focus on clean separation: API returns JSON, frontend consumes it
- Don't over-engineer - this is MVP
- If something is unclear, pick the simpler option
- The `go:embed` directive must be in a `.go` file in the same package as the embedded files' parent directory
