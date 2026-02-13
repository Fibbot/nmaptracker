# Web API and Frontend Architecture

## Router and Hosting Model
`internal/web/server.go` registers:
- `/api/*`: JSON API endpoints.
- `/*`: embedded static frontend files from `internal/web/frontend/*`.

The server listens on localhost (`127.0.0.1`) when launched by CLI `serve`.

## API Surface by Domain
### Project and host/port workflow
- project CRUD/stats
- host list/detail/notes/delete/latest-scan
- port list/status/notes
- bulk host/port status updates

### Scope
- list/add/delete scope rules
- scope evaluation endpoint

### Imports and analytics
- upload import (`POST /projects/{id}/import`)
- list imports and intents
- set import intents
- coverage matrix + missing drilldown
- import delta comparison
- expected baseline CRUD + evaluation
- service campaign queues

### Export
- project export endpoint
- host export endpoint

## Request Security Model
Mutating API routes pass through `csrfGuard`:
- allows requests with no `Origin` header (CLI/curl style)
- validates `Origin` host is local (`localhost` or `127.0.0.1`)
- validates request host is local
- blocks cross-origin browser writes

This is a local-app guard, not full authn/authz.

## Frontend Structure
### Pages
- `index.html`: project list/create landing
- `project.html`: dashboard and project-level actions
- `hosts.html`: host inventory and filters
- `host.html`: host details and ports
- `scan_results.html`: import-focused browsing
- `coverage_matrix.html`: intent coverage matrix
- `import_delta.html`: import-to-import delta
- `service_queues.html`: campaign queues

### JavaScript modules
- `js/projects.js`, `js/dashboard.js`, `js/hosts.js`, `js/host.js`
- `js/scan_results.js`, `js/coverage_matrix.js`, `js/import_delta.js`, `js/service_queues.js`
- shared helpers in `js/app.js`

### Styling
- global stylesheet: `css/style.css`
- no framework and no transpile/build step

## Data Contract Expectations
- API responses are JSON and mostly explicit structs from handlers.
- Frontend assumes stable key names for intent labels, counts, and queue summaries.
- Coverage intent ordering is fixed by backend (`internal/db/intents.go`).
- Import list payload includes source-tracking metadata fields:
  - `nmap_args`
  - `scanner_label`
  - `source_ip`
  - `source_port`
  - `source_port_raw`
- Import upload accepts optional multipart fields:
  - `scanner_label`
  - `source_ip` (IPv4)
  - `source_port` (1-65535)

## Practical Extension Pattern
For a new UI feature:
1. Add DB query/service methods.
2. Add handler + route in `internal/web`.
3. Add new page and JS module under `internal/web/frontend`.
4. Wire navigation and query params from existing dashboard/project pages.
5. Add coverage in `internal/web/handlers_test.go` and subsystem tests.

## Related Files
- `internal/web/server.go`
- `internal/web/handlers.go`
- `internal/web/scope_handlers.go`
- `internal/web/imports_handlers.go`
- `internal/web/coverage_handlers.go`
- `internal/web/delta_handlers.go`
- `internal/web/baseline_handlers.go`
- `internal/web/queue_handlers.go`
