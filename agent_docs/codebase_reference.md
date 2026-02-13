# NmapTracker Codebase Reference

## Purpose
This file is the entrypoint to deep architecture documentation.
It stays concise and points to source-of-truth deep dives under `agent_docs/`.

## Start Here (Recommended Read Order)
1. `agent_docs/codebase_architecture.md`
2. `agent_docs/data_model_and_migrations.md`
3. `agent_docs/import_pipeline_and_intents.md`
4. `agent_docs/web_api_and_frontend.md`
5. `agent_docs/testing_and_quality_map.md`

## Current Implementation Scope
Implemented feature set includes completed work tracked in:
- `features/done/01_shared_data_model_and_api_foundation.md`
- `features/done/02_coverage_matrix_by_scan_type_implementation.md`
- `features/done/03_gap_dashboard_milestones_implementation.md`
- `features/done/04_import_delta_view_implementation.md`
- `features/done/05_expected_asset_baseline_implementation.md`
- `features/done/06_service_campaign_queues_implementation.md`

## Quick Source Anchors
- CLI entry: `cmd/nmap-tracker/main.go`
- DB bootstrap/migrations: `internal/db/db.go`, `internal/db/migrations/*.sql`
- Import orchestration: `internal/importer/importer.go`
- API/router: `internal/web/server.go`
- Frontend assets: `internal/web/frontend/*`

## Notes
- Keep deep architecture details in `agent_docs/*` files listed above.
- Keep `AGENTS.md` short/stable and focused on execution guidance.
