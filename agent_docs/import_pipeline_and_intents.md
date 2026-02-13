# Import Pipeline and Intent System

## Import Entry Points
### CLI import
- Command: `nmap-tracker import <xml-file> --project <name> [--db <path>]`
- Flow in `cmd/nmap-tracker/main.go`:
  - resolve project
  - build matcher with `scope.NewMatcher(nil)` (allow-all)
  - call `importer.ImportXMLFile(...)`

### Web import
- Route: `POST /api/projects/{id}/import`
- Flow in `internal/web/scope_handlers.go`:
  - parse multipart file upload (200MB max)
  - load project scope definitions
  - build matcher from scope rules
  - collect manual intents from form values
  - call `importer.ImportXMLWithOptions(...)`

## Import Execution Path
Main orchestration is in `internal/importer/importer.go`.

### Transactional sequence
1. Start DB transaction.
2. Insert `scan_import` row.
3. Resolve intent set and persist `scan_import_intent`.
4. Parse hosts/ports from XML stream.
5. Validate host IP (IPv4 for streaming path).
6. Upsert host and port current-state rows.
7. Insert `host_observation` and `port_observation`.
8. Update import host/port counts.
9. Commit transaction.

If any step fails, the transaction rolls back.

## Intent Model
### Supported intents
- `ping_sweep`
- `top_1k_tcp`
- `all_tcp`
- `top_udp`
- `vuln_nse`

Defined in `internal/db/intents.go`.

### Intent sources
- `manual`: explicitly provided by user/API payload.
- `auto`: inferred from filename and Nmap arguments.

### Inference heuristics
Auto inference inspects Nmap args and filename patterns, including:
- `-sn` or `ping` naming hints -> `ping_sweep`
- `--top-ports 1000` or likely default-port scan -> `top_1k_tcp`
- `-p-` / full port range -> `all_tcp`
- `-sU` with top/default port behavior -> `top_udp`
- `--script vuln` -> `vuln_nse`

Manual intents override duplicate auto suggestions in final resolved output.

## Scope Interaction
The matcher determines per-host `in_scope` state during import.
- Empty scope rule set means all hosts are considered in scope.
- Invalid scope definitions are ignored by matcher construction.

## Latest Scan Synchronization
When import intents are updated via API (`PUT /imports/{importID}/intents`),
`internal/db/scan_import.go` recalculates `host.latest_scan` for hosts observed in that import.

Current derived labels prioritize:
1. `all_tcp` -> full port
2. `top_1k_tcp` -> top-1k
3. `ping_sweep` -> ping
4. fallback -> none

## Operational Pitfalls
- CLI import currently uses allow-all scope matcher (`scope.NewMatcher(nil)`), unlike web import which uses stored scope definitions.
- Non-IPv4 host records can be skipped in streaming import path.
- Intent updates are authoritative replacement, not patch/merge.

## Related Files
- `cmd/nmap-tracker/main.go`
- `internal/importer/importer.go`
- `internal/importer/xml.go`
- `internal/db/intents.go`
- `internal/db/scan_import.go`
- `internal/web/scope_handlers.go`
