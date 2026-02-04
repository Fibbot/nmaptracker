# Coverage Matrix by Scan Type - Implementation Spec

## Goal
Show coverage of scan intents (`ping_sweep`, `top_1k_tcp`, `all_tcp`, `top_udp`, `vuln_nse`) by scope segment, including what is missing.

## Dependencies
- Requires `scan_import_intent`, `host_observation`, and imports/intents APIs from `features/01_shared_data_model_and_api_foundation.md`.

## 1) Segment model (locked)

## Primary segmentation
Use **scope definitions first**:
- Each scope definition (`scope_definition`) is one segment.
- Segment key: `scope:{scope_definition.id}`.
- Segment label: raw `definition` (`10.0.0.0/24`, `192.168.1.20`, etc.).

## Membership rules
- For CIDR rule: host belongs if `prefix.Contains(host.ip_address)`.
- For IP rule: host belongs if exact IP matches.
- Segment population source: current `host` table (`project_id`, `in_scope=1`).

## Unmapped safety segment
When scope rules exist, add synthetic segment:
- key: `scope:unmapped`
- label: `In-scope (unmapped)`
- membership: in-scope hosts not matching any scope definition.

## Fallback segmentation (no scope rules)
If project has zero scope definitions:
- Bucket in-scope hosts by `/24` from `host.ip_int`.
- Segment key: `fallback:10.20.30.0/24`.
- Segment label: same CIDR string.

## 2) Coverage computation

For each segment and each intent column:

### Host-set definitions
- `segment_hosts`: current in-scope hosts in that segment.
- `covered_hosts(intent)`: hosts in `segment_hosts` that have at least one `host_observation` joined to `scan_import_intent.intent = intent`.

### Metrics per cell
- `covered_count`
- `total_count`
- `coverage_percent = floor(covered_count * 100 / total_count)` (0 when total=0)
- `missing_count = total_count - covered_count`
- `missing_hosts` (paginated list for drill-down)

## Intent columns (fixed order)
1. `ping_sweep`
2. `top_1k_tcp`
3. `all_tcp`
4. `top_udp`
5. `vuln_nse`

Display labels:
- Ping Sweep
- Top 1k TCP
- All TCP
- Top UDP
- Vuln NSE

## 3) API contract

### Endpoint
`GET /api/projects/{id}/coverage-matrix`

### Query params
- `include_missing_preview` (default `true`)
- `missing_preview_size` (default `5`, max `50`)

### Response
```json
{
  "generated_at": "2026-02-04T19:10:00Z",
  "project_id": 3,
  "segment_mode": "scope_rules",
  "intents": ["ping_sweep","top_1k_tcp","all_tcp","top_udp","vuln_nse"],
  "segments": [
    {
      "segment_key": "scope:14",
      "segment_label": "10.0.10.0/24",
      "host_total": 24,
      "cells": {
        "ping_sweep": {
          "covered_count": 24,
          "missing_count": 0,
          "coverage_percent": 100,
          "missing_hosts": []
        },
        "top_1k_tcp": {
          "covered_count": 20,
          "missing_count": 4,
          "coverage_percent": 83,
          "missing_hosts": [
            {"ip_address":"10.0.10.44","host_id":889,"hostname":""}
          ]
        }
      }
    }
  ]
}
```

## Drill-down route (optional but recommended)
`GET /api/projects/{id}/coverage-matrix/missing?segment_key=...&intent=...&page=...&page_size=...`

## 4) Query and performance plan

## SQL approach
Use CTE pipeline:
1. `segment_membership(project_id, segment_key, host_id, ip_address)` from scope/fallback strategy.
2. `intent_host_coverage(project_id, intent, ip_address)` from `host_observation` + `scan_import_intent`.
3. Aggregate with left joins to compute counts.

## Required indexes used
- `idx_host_in_scope`
- `idx_host_observation_project_ip`
- `idx_scan_import_intent_intent`
- `idx_scan_import_intent_scan_import`
- `idx_host_ip_int` (fallback /24)

## 5) Backend implementation map

### DB layer
Add `internal/db/coverage_matrix.go`:
- `GetCoverageMatrix(projectID int64, opts CoverageMatrixOptions) (CoverageMatrixResponse, error)`

### Web layer
- Register route in `internal/web/server.go`:
  - `GET /api/projects/{id}/coverage-matrix`
- Add handler in new file `internal/web/coverage_handlers.go`.

## 6) Frontend implementation

## New page
- `internal/web/frontend/coverage_matrix.html`
- `internal/web/frontend/js/coverage_matrix.js`

## Dashboard link
- Add button in `project.html` and set href in `dashboard.js`:
  - `coverage_matrix.html?id={projectId}`

## UI behavior
- Matrix table: rows = segments, columns = intents.
- Cell visual:
  - percentage + covered/total
  - red badge when missing > 0
- Clicking cell opens missing-host modal with link to `hosts.html?id={projectId}` filtered by subnet where possible.

## 7) Acceptance criteria
- Matrix reflects all five intent columns in fixed order.
- Segment mode follows locked rule (scope first, /24 fallback).
- Missing hosts per cell are accurate and drill-down works.
- Handles empty projects and zero in-scope hosts gracefully.

## 8) Tests
- Unit:
  - Scope segment membership resolution (IP + CIDR + unmapped).
  - /24 fallback segmentation.
  - Coverage counts for each intent.
- API:
  - Response shape and values for mixed coverage dataset.
  - Empty dataset behavior.
- UI smoke:
  - Page renders matrix and modal with mocked API payload.
