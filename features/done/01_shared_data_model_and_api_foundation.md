# Shared Data Model and API Foundation

## Goal
Define the shared schema, importer, DB-query, and API contracts required by all feature docs (`02`-`06`).

## Locked defaults
- Intent assignment mode: **manual + auto-suggest**.
- Coverage segments: **scope rules first**, with **observed `/24` fallback**.
- Delta compare mode: **any two imports**.
- Milestone progression: **`ping_sweep` -> `top_1k_tcp` -> `all_tcp`**.

## 1) Database migration plan

## New migration file
Create `internal/db/migrations/003_feature_foundation.sql`.

### 1.1 `scan_import_intent`
```sql
CREATE TABLE IF NOT EXISTS scan_import_intent (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_import_id INTEGER NOT NULL,
    intent TEXT NOT NULL,
    source TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(scan_import_id) REFERENCES scan_import(id) ON DELETE CASCADE,
    UNIQUE(scan_import_id, intent)
);

CREATE INDEX IF NOT EXISTS idx_scan_import_intent_scan_import ON scan_import_intent(scan_import_id);
CREATE INDEX IF NOT EXISTS idx_scan_import_intent_intent ON scan_import_intent(intent);
```

`intent` allowed values (validated in app layer):
- `ping_sweep`
- `top_1k_tcp`
- `all_tcp`
- `top_udp`
- `vuln_nse`

`source` allowed values:
- `manual`
- `auto`

### 1.2 `host_observation`
```sql
CREATE TABLE IF NOT EXISTS host_observation (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_import_id INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    ip_address TEXT NOT NULL,
    hostname TEXT,
    in_scope BOOLEAN NOT NULL,
    host_state TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(scan_import_id) REFERENCES scan_import(id) ON DELETE CASCADE,
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE,
    UNIQUE(scan_import_id, ip_address)
);

CREATE INDEX IF NOT EXISTS idx_host_observation_project ON host_observation(project_id);
CREATE INDEX IF NOT EXISTS idx_host_observation_project_ip ON host_observation(project_id, ip_address);
CREATE INDEX IF NOT EXISTS idx_host_observation_import ON host_observation(scan_import_id);
```

### 1.3 `port_observation`
```sql
CREATE TABLE IF NOT EXISTS port_observation (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_import_id INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    ip_address TEXT NOT NULL,
    port_number INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    state TEXT NOT NULL,
    service TEXT,
    version TEXT,
    product TEXT,
    extra_info TEXT,
    script_output TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(scan_import_id) REFERENCES scan_import(id) ON DELETE CASCADE,
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE,
    UNIQUE(scan_import_id, ip_address, port_number, protocol)
);

CREATE INDEX IF NOT EXISTS idx_port_observation_project ON port_observation(project_id);
CREATE INDEX IF NOT EXISTS idx_port_observation_project_ip ON port_observation(project_id, ip_address);
CREATE INDEX IF NOT EXISTS idx_port_observation_import ON port_observation(scan_import_id);
CREATE INDEX IF NOT EXISTS idx_port_observation_open ON port_observation(project_id, state, ip_address);
```

### 1.4 `expected_asset_baseline`
```sql
CREATE TABLE IF NOT EXISTS expected_asset_baseline (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    definition TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE,
    UNIQUE(project_id, definition)
);

CREATE INDEX IF NOT EXISTS idx_expected_asset_baseline_project ON expected_asset_baseline(project_id);
```

## 2) Shared Go types and constants

## Add to `internal/db/models.go`
- `type ScanImportIntent struct { ... }`
- `type HostObservation struct { ... }`
- `type PortObservation struct { ... }`
- `type ExpectedAssetBaseline struct { ... }`

## Add intent constants (new file `internal/db/intents.go`)
- `IntentPingSweep`, `IntentTop1KTCP`, `IntentAllTCP`, `IntentTopUDP`, `IntentVulnNSE`
- `ValidIntent(intent string) bool`

## Add source constants
- `IntentSourceManual`, `IntentSourceAuto`

## 3) Import pipeline changes

## 3.1 Parser changes in `internal/importer/xml.go`
Extend XML structs to expose:
- `<nmaprun args="...">` (scan command args)
- `<host><status state="...">` (host state)

Add parser output metadata type:
- `type ParseMetadata struct { NmapArgs string }`

Update observation mapping:
- Include `HostState` in `HostObservation`.

## 3.2 Import options and intent capture in `internal/importer/importer.go`
Introduce:
```go
type ImportOptions struct {
    ManualIntents []string
}
```

New functions:
- `SuggestIntents(filename string, nmapArgs string, obs Observations) []SuggestedIntent`
- `ResolveImportIntents(manual []string, suggested []SuggestedIntent) []db.ScanImportIntent`

Auto-suggest rules (locked):
- `-sn` => `ping_sweep`
- `--top-ports 1000` => `top_1k_tcp`
- `-p-` or full TCP service range => `all_tcp`
- `-sU` + top/default UDP pattern => `top_udp`
- `--script vuln` => `vuln_nse`

Manual precedence:
- Manual intents are always inserted with `source='manual', confidence=1.0`.
- Auto intents inserted only when not duplicated by manual.

## 3.3 Transactional writes
Within the same import transaction:
1. Insert `scan_import` row.
2. Insert `scan_import_intent` rows.
3. For each valid host:
   - Upsert current host state (`host`).
   - Insert one `host_observation` row keyed by `(scan_import_id, ip_address)`.
4. For each port observation:
   - Upsert current port state (`port`).
   - Insert one `port_observation` row keyed by `(scan_import_id, ip_address, port_number, protocol)`.
5. Update `scan_import.hosts_found/ports_found` and commit.

## 4) Shared DB helper additions (`internal/db/*`)

## 4.1 Import/intents helpers
- `ListScanImports(projectID int64)` remains.
- Add `ListScanImportsWithIntents(projectID int64) ([]ScanImportWithIntents, error)`.
- Add `SetScanImportIntents(projectID, importID int64, intents []ScanImportIntentInput) error`.
  - Implementation: delete existing intents for that import; insert validated set.

## 4.2 Observation helpers
- `InsertHostObservation(tx, HostObservation)`
- `InsertPortObservation(tx, PortObservation)`
- `ListHostObservationsByImport(projectID, importID)`
- `ListPortObservationsByImport(projectID, importID)`

## 4.3 Baseline helpers
- `ListExpectedAssetBaselines(projectID)`
- `BulkAddExpectedAssetBaselines(projectID, defs []string)`
- `DeleteExpectedAssetBaseline(projectID, baselineID)`

## 5) Shared API endpoints and contracts

## 5.1 Imports + intents
### `GET /api/projects/{id}/imports`
Response:
```json
{
  "items": [
    {
      "id": 12,
      "project_id": 3,
      "filename": "scan_2026-02-03.xml",
      "import_time": "2026-02-03T21:13:00Z",
      "hosts_found": 145,
      "ports_found": 934,
      "intents": [
        {"intent": "top_1k_tcp", "source": "auto", "confidence": 0.98},
        {"intent": "vuln_nse", "source": "manual", "confidence": 1.0}
      ]
    }
  ],
  "total": 1
}
```

### `PUT /api/projects/{id}/imports/{importID}/intents`
Request:
```json
{
  "intents": [
    {"intent": "top_1k_tcp", "source": "manual", "confidence": 1.0},
    {"intent": "vuln_nse", "source": "manual", "confidence": 1.0}
  ]
}
```
Response: `{"status":"ok"}`

Validation:
- Must belong to `{id}` project.
- `intent` in allowed list.
- `source` in `manual|auto`.
- `confidence` between `0` and `1`.

## 5.2 Feature endpoints (to be implemented in feature docs)
- `GET /api/projects/{id}/coverage-matrix`
- `GET /api/projects/{id}/gaps`
- `GET /api/projects/{id}/delta?base_import_id=&target_import_id=`
- `GET /api/projects/{id}/baseline`
- `POST /api/projects/{id}/baseline`
- `DELETE /api/projects/{id}/baseline/{baselineID}`
- `GET /api/projects/{id}/baseline/evaluate`
- `GET /api/projects/{id}/queues/services`
- `GET /api/projects/{id}/queues/milestones`

## 6) Backward compatibility and rollout sequence

## Step order (mandatory)
1. Add migration `003_feature_foundation.sql`.
2. Add DB models/constants/helpers.
3. Extend importer parser + import transaction writes.
4. Add imports/intents API endpoints.
5. Add feature-specific query endpoints.
6. Add frontend pages/components.
7. Add tests.

## Compatibility guarantees
- Existing endpoints remain unchanged.
- Existing imports stay valid; historical imports will have empty observation/intents until reimported.
- No destructive migration of existing tables.

## 7) Shared test plan

### Migration tests
- New tables/indexes/uniques/FKs exist.
- Migration idempotent on repeated `db.Open(...)`.

### Import tests
- Manual intents persisted.
- Auto intents inferred from nmap args.
- Manual+auto dedupe behavior.
- Host/port observations inserted per import.
- Transaction rollback removes observations/intents if import fails.

### API tests
- `GET /imports` includes intents.
- `PUT /imports/{importID}/intents` validates project scoping and enums.

### Regression tests
- Existing project/host/port CRUD and export flows still pass unchanged.
