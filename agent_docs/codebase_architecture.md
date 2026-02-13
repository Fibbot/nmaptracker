# Codebase Architecture Deep Dive

## Goals and Shape
Nmap Tracker is a monolithic Go application that combines:
- A CLI for project lifecycle, imports, and exports.
- An embedded web server for interactive analysis workflows.
- A local SQLite datastore with embedded schema migrations.

The architecture prioritizes simple local operation, deterministic behavior, and low setup overhead.

## Top-Level Module Map
- `cmd/nmap-tracker/main.go`: CLI command parsing and process lifecycle.
- `internal/db/*`: data access, migrations, reporting queries, and domain-level persistence logic.
- `internal/importer/*`: Nmap parsing, intent inference, and import orchestration.
- `internal/scope/*`: scope rule parsing and matching.
- `internal/web/*`: HTTP API handlers, router wiring, and embedded static assets.
- `internal/export/*`: JSON/CSV/TXT export writers.

## Runtime Composition
### CLI runtime
`run()` dispatches one of:
- `serve`: opens DB, builds `web.Server`, listens on `127.0.0.1:<port>`.
- `projects`: list/create projects.
- `import`: imports one XML file into an existing project.
- `export`: writes project exports in JSON/CSV.

### Web runtime
`internal/web/server.go` constructs a single router:
- `/api/*` JSON endpoints for CRUD and analysis views.
- `/*` static file handler from embedded `internal/web/frontend/*`.

The API uses a local-host origin guard (`csrfGuard`) for non-GET browser requests.

## Core Data-Flow Patterns
### Import path (CLI and Web)
1. Open DB and begin transaction.
2. Insert `scan_import` metadata row.
3. Resolve/import intents (manual + inferred).
4. Parse host/port observations.
5. Upsert current state (`host`, `port`).
6. Insert per-import history (`host_observation`, `port_observation`).
7. Update import counts and commit.

This path is implemented in `internal/importer/importer.go` and relies on DB helpers in `internal/db/*`.

### Analytics/read path
Coverage matrix, delta, baseline, and service queues are computed from persisted data, mainly via:
- Observation tables for time-aware analysis.
- Current-state tables for host/port workflow and metadata.

### Export path
Export endpoints and CLI call `internal/export/*` writers over DB results:
- Project-level JSON/CSV.
- Host-level JSON/CSV/TXT via web endpoints.

## Architectural Invariants
- SQLite is authoritative for all state; there is no external cache.
- Schema migrations are embedded and executed on DB open.
- Import writes are transactional.
- Scope matching defaults to allow-all when no scope definitions exist.
- Web server binds to localhost and validates browser origin/host for mutating API requests.

## Coupling Boundaries
- `internal/web` depends on `internal/db`, `internal/importer`, and `internal/scope`.
- `internal/importer` depends on `internal/db` and `internal/scope`, but not on `internal/web`.
- `cmd/nmap-tracker` orchestrates modules and keeps command semantics at the edge.

## Where to Extend
- New analytics view: add query/service in `internal/db`, expose handler in `internal/web`, add frontend page+JS.
- New import metadata: extend importer parsing + DB schema/query writes.
- New export format: add writer in `internal/export` and route/CLI wiring.

## Related Deep Docs
- `agent_docs/data_model_and_migrations.md`
- `agent_docs/import_pipeline_and_intents.md`
- `agent_docs/web_api_and_frontend.md`
- `agent_docs/testing_and_quality_map.md`
