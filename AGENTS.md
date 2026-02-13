# AGENTS.md

This file provides guidance to OpenAI Codex when working with code in this repository.
This is the always-in-context guide for coding agents in this repo.
Keep this file short and stable. Put deep architecture and rationale in `agent_docs/`.

## Project Overview
Nmap Tracker is a self-hosted Nmap scan tracking tool with both CLI and web UI workflows.
It imports Nmap XML into per-project datasets, tracks current host/port state plus historical observations, and supports scope, intent, coverage, delta, baseline, and service queue analysis.
Per-import scanner source metadata is persisted on `scan_import` (`nmap_args`, `scanner_label`, `source_ip`, `source_port`, `source_port_raw`).

## Project Context
The preferred local workflow is:
1. Build binary.
2. Run `serve` on localhost.
3. Manage projects/imports/analysis via the web UI.

CLI remains first-class for project creation, import, and export automation.
Current implemented feature docs are in `features/done/01` through `features/done/06`.

## Tech Stack
- Language/runtime: Go `1.24.12`.
- DB: SQLite via `modernc.org/sqlite` (embedded migrations, WAL mode enabled by DB bootstrap).
- HTTP router: `github.com/go-chi/chi/v5`.
- Frontend: embedded static HTML/CSS/vanilla JS from `internal/web/frontend/*` (no frontend build pipeline).

## Core Architecture
- `cmd/nmap-tracker/main.go`: CLI entrypoint (`serve`, `projects`, `import`, `export`).
- `internal/db/*`: data layer, migrations, and query modules.
- `internal/importer/*`: XML/GNMAP ingestion and import intent handling.
- Import pipeline also resolves scanner source metadata from `nmaprun.args` flags (`-S`, `-g`, `--source-port`) with manual fallback from CLI/web import options.
- `internal/scope/*`: include-list scope matcher.
- `internal/web/*`: API handlers + static asset hosting.
- `agent_docs/codebase_reference.md`: deep-doc index and read order.
- `agent_docs/codebase_architecture.md`: runtime/module architecture.
- `agent_docs/data_model_and_migrations.md`: schema, migrations, DB invariants.
- `agent_docs/import_pipeline_and_intents.md`: ingestion + intent resolution behavior.
- `agent_docs/web_api_and_frontend.md`: API surface and frontend wiring.
- `agent_docs/testing_and_quality_map.md`: automated test coverage map and quality checks.

### Key Design Patterns
- Current-state plus history:
  - `host` and `port` hold latest merged state.
  - `scan_import`, `host_observation`, and `port_observation` preserve per-import history.
- Import intent classification (`scan_import_intent`) is used for coverage and queue workflows.
- Scope behavior is allow-list oriented; empty scope rules imply in-scope.
- Web API enforces local-origin protection for browser write operations.

## Database Schema (5 migrations)
- `001_init.sql`: core tables (`project`, `scope_definition`, `scan_import`, `host`, `port`).
- `002_add_host_ip_int.sql`: integer IP support for range/subnet operations.
- `003_feature_foundation.sql`: intents, observations, expected asset baseline.
- `004_add_host_latest_scan.sql`: `host.latest_scan` column.
- `005_remove_parking_lot_status.sql`: normalizes legacy `parking_lot` to `flagged`.

Deep schema behavior and query semantics belong in the deep-doc set linked above, starting at `agent_docs/codebase_reference.md`.

## Development Commands
- Test: `make test`
- Build: `make build`
- Cross-builds: `make build-all`
- Run UI server: `./nmap-tracker serve --port 8080 --db nmap-tracker.db`
- List projects: `./nmap-tracker projects list --db nmap-tracker.db`
- Create project: `./nmap-tracker projects create <name> --db nmap-tracker.db`
- Import XML: `./nmap-tracker import <scan.xml> --project <name> [--scanner-label <label>] [--source-ip <ipv4>] [--source-port <1-65535>] --db nmap-tracker.db`
- Export project: `./nmap-tracker export --project <name> --output <file> --format <json|csv> --db nmap-tracker.db`

## Common Queries
- Key API routes live under `/api/projects/{id}`:
  - `/imports`, `/coverage-matrix`, `/coverage-matrix/missing`, `/delta`
  - `/baseline`, `/baseline/evaluate`
  - `/queues/services`
- Quick DB sanity checks:
  - `sqlite3 nmap-tracker.db "select id, name from project order by id;"`
  - `sqlite3 nmap-tracker.db "select project_id, count(*) from scan_import group by project_id;"`
  - `sqlite3 nmap-tracker.db "select project_id, count(*) from host group by project_id;"`

## Next Implementation Steps
1. Keep `AGENTS.md` updated whenever architecture, commands, or workflow expectations change.
2. Put deep implementation notes and rationale updates in the deep-doc set under `agent_docs/` and maintain `agent_docs/codebase_reference.md` as the index.
3. For new features, add or update tests in the touched package (`internal/db`, `internal/importer`, `internal/web`, `cmd/nmap-tracker`) before closing work.
4. Preserve backward-compatible API behavior unless a feature brief explicitly calls for breaking changes.

---

ALWAYS update the AGENTS.md with any new or relevant information from our settings.

As the last stage of implementing a stage, always use agent_docs/agent_staffEngineer.md to verify and assure your work is up to our standards.

Utilize agent_docs/agent_* for their related guidance when needed.

Always output a "testPlan.md" (reference file at agent_docs/testPlan.md) for the manual tester to run through and verify fixes and or updates.

### Memory and Learning
- At the start of every session, read `agent_docs/lessons.md` to internalize project-specific patterns and past mistakes.
- Whenever I provide a correction or you discover a non-obvious solution, propose a new entry for `agent_docs/lessons.md` summarizing the failure mode and the "lesson" to prevent it.
- Keep `agent_docs/lessons.md` concise and high-signal; prioritize patterns over one-off instances.
