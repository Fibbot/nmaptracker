# Data Model and Migrations

## Persistence Model
The data model intentionally keeps both:
- Latest merged operational state for workflow UI.
- Per-import historical observations for time-based analysis.

This split enables fast operational edits while preserving import history.

## Core Tables
### Project and scope
- `project`: top-level container.
- `scope_definition`: project scope entries (`definition`, `type`).

### Import metadata
- `scan_import`: one row per imported file.
- `scan_import_intent`: intent tags attached to an import.
- `scan_import` source metadata fields:
  - `nmap_args`
  - `scanner_label`
  - `source_ip`
  - `source_port`
  - `source_port_raw`

### Current merged state
- `host`: canonical host row per `project_id + ip_address`.
- `port`: canonical port row per `host_id + port_number + protocol`.

### Historical observations
- `host_observation`: host snapshot for one `scan_import`.
- `port_observation`: port snapshot for one `scan_import`.

### Baseline inventory
- `expected_asset_baseline`: expected IP/CIDR definitions per project.

## Key Constraints and Indexes
- `host` unique key: `(project_id, ip_address)`.
- `port` unique key: `(host_id, port_number, protocol)`.
- Observation uniqueness:
  - `host_observation`: `(scan_import_id, ip_address)`.
  - `port_observation`: `(scan_import_id, ip_address, port_number, protocol)`.
- IPv4 integer sort/filter support via `host.ip_int` and index `idx_host_ip_int`.

## Migration Timeline
### `001_init.sql`
Creates base domain tables and supporting indexes:
- `project`, `scope_definition`, `scan_import`, `host`, `port`.

### `002_add_host_ip_int.sql`
Adds:
- `host.ip_int` for stable numeric IPv4 ordering and subnet/range filtering.

### `003_feature_foundation.sql`
Adds feature foundation tables:
- `scan_import_intent`
- `host_observation`
- `port_observation`
- `expected_asset_baseline`

### `004_add_host_latest_scan.sql`
Adds:
- `host.latest_scan` (default `none`) for quick host scan-depth labeling.

### `005_remove_parking_lot_status.sql`
Normalizes legacy work status values:
- maps `parking_lot` to `flagged` in `port.work_status`.

### `006_add_scan_import_source_metadata.sql`
Adds source-tracking fields on `scan_import`:
- raw command args (`nmap_args`)
- operator scanner label (`scanner_label`)
- canonical source metadata (`source_ip`, `source_port`)
- raw unparsed source-port token (`source_port_raw`)

## DB Open Behavior
`internal/db/db.go` applies runtime DB initialization:
- `PRAGMA busy_timeout = 5000`
- `PRAGMA foreign_keys = ON`
- `PRAGMA journal_mode = WAL`
- run embedded migrations in lexical filename order
- ensure `ip_int` index and backfill missing `host.ip_int` values

## Behavioral Rules Worth Preserving
- Coverage/delta analytics depend on observation tables, not just current merged state.
- Service queues filter to in-scope hosts and open/open|filtered ports.
- Baseline definitions are IPv4-only; CIDR broader than `/16` is rejected.
- Updating import intents can trigger host `latest_scan` synchronization based on most recent observed import intents.

## Related Files
- `internal/db/migrations/*.sql`
- `internal/db/db.go`
- `internal/db/models.go`
- `internal/db/scan_import.go`
- `internal/db/baseline.go`
- `internal/db/service_queues.go`
