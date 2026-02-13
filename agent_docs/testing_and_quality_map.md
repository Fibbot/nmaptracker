# Testing and Quality Map

## Test Suite Layout
### CLI
- `cmd/nmap-tracker/main_test.go`

### Database and analytics
- `internal/db/db_test.go`
- `internal/db/crud_test.go`
- `internal/db/workflow_test.go`
- `internal/db/coverage_matrix_test.go`
- `internal/db/delta_test.go`
- `internal/db/baseline_test.go`
- `internal/db/service_queues_test.go`

### Importing and parsing
- `internal/importer/xml_test.go`
- `internal/importer/intents_test.go`
- `internal/importer/importer_test.go`

### Web/API
- `internal/web/handlers_test.go`

### Utilities and exports
- `internal/export/export_test.go`
- `internal/scope/matcher_test.go`
- `internal/testutil/tempdir_test.go`

## What Is Verified Well
- Migration/bootstrap behavior and core CRUD paths.
- Import parsing, intent inference, and persistence flow.
- Coverage matrix, delta, baseline, and service queue query correctness.
- API route behavior for major project workflows.

## Common Gaps to Watch
- Frontend behavior is mostly integration-tested indirectly (no JS unit test harness).
- Security model is local-origin focused; avoid assuming external hardening.
- Subtle regressions can appear when changing data-shape contracts for dashboard pages.

## Change Checklist for Contributors
1. Add or update tests in the package you changed.
2. Run `make test` locally.
3. If touching route payloads, update API handler tests.
4. If touching import semantics, update importer + DB analytics tests together.
5. If touching schema/migrations, verify DB open path and backfill behavior.

## Manual Validation
When requested by project guidance, update `agent_docs/testPlan.md` with scenario-driven manual checks for new behavior.
