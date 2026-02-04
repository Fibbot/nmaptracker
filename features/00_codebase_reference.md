# NmapTracker Codebase Reference

## Purpose
This document is the source-of-truth orientation guide for future agents implementing the feature specs in `features/01` through `features/06`.

## Repository layout (current)

### CLI entrypoint
- `cmd/nmap-tracker/main.go`
  - Subcommands: `serve`, `projects`, `import`, `export`.
  - CLI import path currently calls `importer.ImportXMLFile(...)` with `scope.NewMatcher(nil)` (all in-scope).

### Database layer
- `internal/db/db.go`
  - Opens SQLite DB, enables WAL/foreign keys, runs migrations.
- `internal/db/migrations/001_init.sql`
  - Tables: `project`, `scope_definition`, `scan_import`, `host`, `port`.
- `internal/db/migrations/002_add_host_ip_int.sql`
  - Adds `host.ip_int` + index for subnet/range filtering.
- Key CRUD/query files:
  - `project.go`, `scope.go`, `scan_import.go`, `host.go`, `port.go`, `host_list.go`, `dashboard.go`, `tx.go`.

### Import pipeline
- `internal/importer/importer.go`
  - Streaming import function: `ImportXML(...)`.
  - Observation import function: `ImportObservations(...)`.
  - Writes one `scan_import` record per upload.
  - Upserts hosts by `(project_id, ip_address)`.
  - Upserts ports by `(host_id, port_number, protocol)`.
- `internal/importer/xml.go`
  - XML parsing structs + `observationFromHost(...)` mapping.

### Scope matching
- `internal/scope/matcher.go`
  - Include-list semantics; no excludes.
  - Empty rules => everything in scope.

### Web/API layer
- `internal/web/server.go`
  - API routes mounted at `/api/*`.
  - Static frontend served from embedded `internal/web/frontend/*`.
- `internal/web/handlers.go`
  - Projects, hosts, ports, bulk status, exports.
- `internal/web/scope_handlers.go`
  - Scope list/add/delete/evaluate + XML import endpoint.

### Frontend
- HTML:
  - `internal/web/frontend/index.html` (projects)
  - `internal/web/frontend/project.html` (dashboard)
  - `internal/web/frontend/hosts.html`
  - `internal/web/frontend/host.html`
  - `internal/web/frontend/scan_results.html`
- JS:
  - `js/projects.js`, `js/dashboard.js`, `js/hosts.js`, `js/host.js`, `js/scan_results.js`, `js/app.js`.

### Export layer
- `internal/export/json.go`, `csv.go`, `text.go`
  - Includes current `scan_import` records in JSON export.

## Current data model behavior and limits

### What is persisted today
- `scan_import`: per-upload metadata (`filename`, `import_time`, aggregate `hosts_found`, `ports_found`).
- `host`: current merged host state per project+IP.
- `port`: current merged port state per host+port+protocol.

### Why import-delta/coverage snapshots are missing
- Imports are additive into **current state** tables.
- There is no table linking a specific host/port observation to a specific `scan_import.id`.
- Historical states are overwritten by upsert; only latest merged values remain.
- Result: cannot currently compute accurate per-import drift, per-import coverage by intent, or milestone completion over time.

## Current import data flow

### Web flow
1. `POST /api/projects/{id}/import` (`internal/web/scope_handlers.go`)
2. Read scope definitions and build matcher.
3. Call `importer.ImportXML(...)` with multipart file stream.
4. Importer inserts `scan_import`, streams each `<host>`, upserts host/ports.
5. API returns aggregate counts (`hosts_imported`, `hosts_skipped`, `ports_imported`).

### CLI flow
1. `nmap-tracker import ...` (`cmd/nmap-tracker/main.go`)
2. Resolve project by name.
3. Use `scope.NewMatcher(nil)`.
4. Call `importer.ImportXMLFile(...)`.

## Existing UI hooks and where new features fit

### Project dashboard (`project.html` + `dashboard.js`)
Current cards:
- Host totals (total/in-scope/out-of-scope).
- Workflow status totals (scanned/flagged/in-progress/done).
- Scope management.
- Import upload.

Recommended insertion points for new features:
- Add feature links/cards below current stats:
  - Coverage Matrix
  - Gap Dashboard
  - Import Delta
  - Expected Baseline
  - Service/Milestone Queues

### Hosts page (`hosts.html` + `hosts.js`)
Current function:
- Filter/paginate hosts with summary status badges.

Feature hook:
- Drill-down landing for queue results and coverage segment host lists.

### All scans page (`scan_results.html` + `scan_results.js`)
Current function:
- Flat current project ports view with status updates.

Feature hook:
- Drill-down landing for “open ports still scanned/flagged”.

## Known constraints
- DB is SQLite via `modernc.org/sqlite`.
- API is plain Go handlers + JSON; no OpenAPI generation.
- Frontend is static JS + server-embedded files (no framework/build tool).
- Current API does **not** expose import-history list endpoint beyond internal DB helpers.
- Existing scope system is allow-list only (no explicit excludes).
- Current import parser supports IPv4 operationally (IPv6 currently filtered in importer path).

## Existing tests relevant to feature work
- DB and migrations:
  - `internal/db/db_test.go`, `crud_test.go`, `workflow_test.go`.
- Importing/parsing:
  - `internal/importer/importer_test.go`, `xml_test.go`.
- API handlers:
  - `internal/web/handlers_test.go`.
- CLI smoke paths:
  - `cmd/nmap-tracker/main_test.go`.

## Implementation implications
- Any feature requiring historical comparisons must introduce per-import observation tables.
- Any feature requiring “scan intent” must add explicit import-intent persistence.
- Shared foundational changes should be completed before feature-specific endpoint/UI work.
