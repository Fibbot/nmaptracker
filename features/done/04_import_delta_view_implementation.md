# Import Delta View - Implementation Spec

## Goal
Compare any two imports and highlight exposure drift so scope changes are not missed.

## Dependencies
- `host_observation`, `port_observation`, `scan_import_intent` from `features/01`.
- Import list endpoint (`GET /api/projects/{id}/imports`).

## 1) Comparison mode (locked)
- User selects any two imports: `base_import_id` and `target_import_id`.
- IDs must belong to same project and must differ.
- Default UI preselect: latest import as target, previous import as base.

## 2) Delta definitions (locked)

## Host deltas
- **net_new_hosts**: in target host observations, absent in base.
- **disappeared_hosts**: in base host observations, absent in target.

## Exposure deltas
Define exposure key: `(ip_address, port_number, protocol)` with port state in `('open','open|filtered')`.

- **net_new_open_exposures**: key in target-open set, absent in base-open set.
- **disappeared_open_exposures**: key in base-open set, absent in target-open set.

## Service fingerprint changes
For keys present and open in both imports:
- fingerprint tuple: `(service, product, version, extra_info)`
- classify as changed when tuple differs.
- Include before/after payload.

## 3) API contract

### Endpoint
`GET /api/projects/{id}/delta?base_import_id={a}&target_import_id={b}`

### Query params
- `preview_size` default `50`, max `500`
- `include_lists` default `true`

### Response
```json
{
  "generated_at": "2026-02-04T19:20:00Z",
  "project_id": 3,
  "base_import": {"id": 70, "filename": "week1.xml", "import_time": "2026-01-20T00:10:00Z"},
  "target_import": {"id": 72, "filename": "week2.xml", "import_time": "2026-01-27T00:10:00Z"},
  "summary": {
    "net_new_hosts": 4,
    "disappeared_hosts": 1,
    "net_new_open_exposures": 9,
    "disappeared_open_exposures": 5,
    "changed_service_fingerprints": 3
  },
  "lists": {
    "net_new_hosts": [{"ip_address":"10.0.4.9","hostname":""}],
    "disappeared_hosts": [{"ip_address":"10.0.2.15","hostname":""}],
    "net_new_open_exposures": [
      {"ip_address":"10.0.4.9","port_number":3389,"protocol":"tcp","state":"open","service":"ms-wbt-server"}
    ],
    "disappeared_open_exposures": [],
    "changed_service_fingerprints": [
      {
        "ip_address":"10.0.3.3",
        "port_number":443,
        "protocol":"tcp",
        "before":{"service":"https","product":"nginx","version":"1.22","extra_info":""},
        "after":{"service":"https","product":"nginx","version":"1.24","extra_info":""}
      }
    ]
  }
}
```

## 4) Query plan

## SQL strategy
- Use CTEs for base/target host sets and base/target open exposure sets.
- Compute add/remove via left anti-joins.
- Compute changed fingerprint via inner join on exposure key and tuple comparison.

## Indexes required
- `idx_host_observation_import`
- `idx_host_observation_project_ip`
- `idx_port_observation_import`
- `idx_port_observation_open`

## 5) Backend implementation map

### DB layer
Create `internal/db/delta.go`:
- `GetImportDelta(projectID, baseImportID, targetImportID int64, opts DeltaOptions) (ImportDeltaResponse, error)`

### Web layer
- Register route:
  - `GET /api/projects/{id}/delta`
- Add handler in `internal/web/delta_handlers.go` with strict validation.

Validation failures:
- missing IDs -> 400
- same IDs -> 400
- import not in project -> 404

## 6) Frontend plan

## New page
- `internal/web/frontend/import_delta.html`
- `internal/web/frontend/js/import_delta.js`

## UX requirements
- Base/target selectors populated from `/imports`.
- "Compare" button triggers `/delta` call.
- Top-level summary cards + tabbed detail lists.
- Export-to-JSON for delta result payload (client-side download).

## Dashboard hook
Add link/button from `project.html` to `import_delta.html?id={projectId}`.

## 7) Acceptance criteria
- Any-two-import comparison works.
- Host and exposure add/remove counts are accurate.
- Fingerprint changes only include keys open in both imports.
- Empty change set is rendered cleanly.

## 8) Tests
- Unit:
  - add/remove host and exposure classification.
  - fingerprint-change detection.
- API:
  - validation and project scoping checks.
  - deterministic response with fixture data.
- UI smoke:
  - default selector behavior (latest vs previous).
