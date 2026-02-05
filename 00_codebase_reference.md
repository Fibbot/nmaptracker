# NmapTracker Codebase Reference

## Purpose
This is the current source-of-truth orientation doc for the implemented `features/done/01` through `features/done/06` work.

## What changed since commit `4c815a3c404a84399012e23f5629b989a8d4b2ac`
- Added Import Delta backend/UI (`internal/db/delta.go`, `internal/web/delta_handlers.go`, `internal/web/frontend/import_delta.html`, `internal/web/frontend/js/import_delta.js`).
- Added Expected Asset Baseline backend/UI (`internal/db/baseline.go`, `internal/web/baseline_handlers.go`, baseline sections in dashboard).
- Added Service Campaign Queue backend/UI (`internal/db/service_queues.go`, `internal/web/queue_handlers.go`, `internal/web/frontend/service_queues.html`, `internal/web/frontend/js/service_queues.js`).
- Added host latest scan support + status cleanup migrations:
  - `internal/db/migrations/004_add_host_latest_scan.sql`
  - `internal/db/migrations/005_remove_parking_lot_status.sql`
- Gap/milestone files introduced in `23c715f` were removed/refactored in `cccebc3`; there is no active `/gaps` or `/queues/milestones` endpoint now.

## Repository layout (current)

### CLI entrypoint
- `cmd/nmap-tracker/main.go`
  - Subcommands: `serve`, `projects`, `import`, `export`.
  - CLI import still uses `scope.NewMatcher(nil)` (all hosts in scope for CLI imports).

### Database layer
- `internal/db/db.go`
  - Opens SQLite, enables WAL/foreign keys, runs embedded migrations.
- Migrations:
  - `001_init.sql` - base tables (`project`, `scope_definition`, `scan_import`, `host`, `port`).
  - `002_add_host_ip_int.sql` - `host.ip_int` for subnet/range filters.
  - `003_feature_foundation.sql` - `scan_import_intent`, `host_observation`, `port_observation`, `expected_asset_baseline`.
  - `004_add_host_latest_scan.sql` - `host.latest_scan`.
  - `005_remove_parking_lot_status.sql` - normalizes legacy `parking_lot` to `flagged`.
- Key query modules:
  - `coverage_matrix.go`, `delta.go`, `baseline.go`, `service_queues.go`
  - plus `scan_import.go`, `observations.go`, `intents.go`, `host_list.go`, `dashboard.go`.

### Import pipeline
- `internal/importer/importer.go`
  - `ImportXMLWithOptions(...)` writes:
    1) `scan_import`
    2) resolved `scan_import_intent` rows (manual + auto-suggest)
    3) current-state upserts (`host`, `port`)
    4) historical rows (`host_observation`, `port_observation`)
  - Auto intent inference uses nmap args/filename signals (`-sn`, `--top-ports 1000`, `-p-`, `-sU`, `--script vuln`).
- `internal/importer/xml.go`
  - XML mapping includes host state + service fingerprint fields used by delta.

### Scope matching
- `internal/scope/matcher.go`
  - Include-list semantics only; empty rules => in scope.

### Web/API layer
- `internal/web/server.go`
  - `/api` routes include:
    - Project/core host/port routes
    - Scope and import upload
    - Imports/intents: `GET /projects/{id}/imports`, `PUT /projects/{id}/imports/{importID}/intents`
    - Coverage: `GET /projects/{id}/coverage-matrix`, `GET /projects/{id}/coverage-matrix/missing`
    - Delta: `GET /projects/{id}/delta`
    - Baseline: `GET|POST /projects/{id}/baseline`, `DELETE /projects/{id}/baseline/{baselineID}`, `GET /projects/{id}/baseline/evaluate`
    - Service queue: `GET /projects/{id}/queues/services`
  - Static frontend served from embedded `internal/web/frontend/*`.

### Frontend
- Main pages:
  - `index.html`, `project.html`, `hosts.html`, `host.html`, `scan_results.html`
  - `coverage_matrix.html`, `import_delta.html`, `service_queues.html`
- Dashboard (`project.html` + `js/dashboard.js`) now includes:
  - import intents editor,
  - a consolidated `Project Tools` hamburger menu for coverage matrix, import delta, service queues, and export actions,
  - a single expand/collapse-all toggle for dashboard sections,
  - import intents bulk save (`Save All`) across listed imports,
  - plain-English helper copy explaining intents drive queue/milestone coverage interpretation (UI guidance only),
  - expected baseline CRUD + evaluate panel.
- Hosts page (`js/hosts.js`) includes editable `latest_scan` per host.

## Current data model behavior

### Persisted historical + current state
- Current merged state:
  - `host` (keyed by `project_id + ip_address`)
  - `port` (keyed by `host_id + port_number + protocol`)
- Per-import history:
  - `scan_import`
  - `scan_import_intent`
  - `host_observation`
  - `port_observation`
- Expected baseline inventory:
  - `expected_asset_baseline`

### Important behavior notes
- Delta and coverage now use per-import observation tables (not inferred from current merged state).
- `host.latest_scan` is updated when intents are set via `PUT /imports/{importID}/intents` (sync logic in `internal/db/scan_import.go`).
- Work statuses are now effectively `scanned | flagged | in_progress | done`.

## Feature behavior summary

### Coverage Matrix
- Segment mode:
  - scope-rule segments first,
  - `/24` fallback when no scope rules.
- Intent columns are fixed by `internal/db/intents.go` order.
- Missing hosts drill-down endpoint supports paging.

### Import Delta
- Any-two-import comparison in same project.
- Tracks:
  - net new/disappeared hosts,
  - net new/disappeared open exposures (`open`, `open|filtered`),
  - service fingerprint changes (`service/product/version/extra_info`).

### Expected Asset Baseline
- Accepts IPv4 IP/CIDR definitions.
- Rejects IPv6 and CIDR broader than `/16`.
- Evaluation computes:
  - expected but unseen
  - seen but outside expected baseline
  - and split by host `in_scope` flag.

### Service Campaign Queues
- Campaigns: `smb`, `ldap`, `rdp`, `http`.
- Includes only in-scope hosts and open/open|filtered ports.
- Returns host-grouped rows with per-status counts and `source_import_ids`.

## Known constraints
- SQLite (`modernc.org/sqlite`) + embedded migrations.
- API is plain Go handlers + JSON.
- Frontend is static HTML/CSS/JS (no build step/framework).
- Scope model is allow-list only (no explicit excludes).
- Import + baseline logic are IPv4-focused.
- No active gap/milestone endpoints in current codebase.
- Any milestone queue wording in the dashboard is descriptive only; there is still no `/gaps` or `/queues/milestones` API endpoint.

## Tests covering implemented feature set
- DB/query/migrations:
  - `internal/db/coverage_matrix_test.go`
  - `internal/db/delta_test.go`
  - `internal/db/baseline_test.go`
  - `internal/db/service_queues_test.go`
  - `internal/db/db_test.go`, `internal/db/crud_test.go`, `internal/db/workflow_test.go`
- Import + intent handling:
  - `internal/importer/importer_test.go`
  - `internal/importer/intents_test.go`
  - `internal/importer/xml_test.go`
- API handlers:
  - `internal/web/handlers_test.go`
- CLI smoke:
  - `cmd/nmap-tracker/main_test.go`
