# Gap Dashboard + Milestones - Implementation Spec

## Goal
Provide explicit operational gap lists and milestone queues:
- In-scope but never scanned.
- Open ports still `scanned`/`flagged`.
- Hosts remaining for `ping_sweep`, `top_1k_tcp`, `all_tcp`.

## Dependencies
- `scan_import_intent`, `host_observation`, `port_observation` foundation from `features/01`.

## 1) Gap definitions (locked)

## A) In-scope but never scanned
A host is in this list when:
- `host.project_id = ?`
- `host.in_scope = 1`
- no row exists in `host_observation` for same `project_id` and `ip_address`.

## B) Open ports still scanned/flagged
Port rows where:
- `host.project_id = ?`
- `host.in_scope = 1`
- `port.state = 'open'`
- `port.work_status IN ('scanned','flagged')`

Output grouped by host and also available as flat rows for scan-results drill-down.

## C) Milestone queues (host-level)
Milestone completion is based on host observations tied to import intents.

### `needs_ping_sweep`
Host is **not complete** unless it has observation in an import with intent in:
- `ping_sweep`
- `top_1k_tcp`
- `all_tcp`

(locked per rule: top1k/all imply ping stage satisfied)

### `needs_top_1k_tcp`
Host is **not complete** unless it has observation in an import with intent in:
- `top_1k_tcp`
- `all_tcp`

(locked per rule: all-ports implies top1k stage satisfied)

### `needs_all_tcp`
Host is **not complete** unless it has observation in an import with intent:
- `all_tcp`

## 2) API contract

### Endpoint
`GET /api/projects/{id}/gaps`

### Query params
- `preview_size` default `10`, max `100`.
- `include_lists` default `true`.

### Response
```json
{
  "generated_at": "2026-02-04T19:15:00Z",
  "project_id": 3,
  "summary": {
    "in_scope_never_scanned": 19,
    "open_ports_scanned_or_flagged": 77,
    "needs_ping_sweep": 14,
    "needs_top_1k_tcp": 23,
    "needs_all_tcp": 91
  },
  "lists": {
    "in_scope_never_scanned": [
      {"host_id": 111, "ip_address": "10.0.1.4", "hostname": ""}
    ],
    "open_ports_scanned_or_flagged": [
      {
        "host_id": 222,
        "ip_address": "10.0.1.9",
        "port_id": 3001,
        "port_number": 445,
        "protocol": "tcp",
        "work_status": "flagged",
        "service": "microsoft-ds"
      }
    ],
    "needs_ping_sweep": [
      {"host_id": 333, "ip_address": "10.0.2.1", "hostname": ""}
    ],
    "needs_top_1k_tcp": [],
    "needs_all_tcp": []
  }
}
```

## Dedicated milestone endpoint
`GET /api/projects/{id}/queues/milestones`
- Returns only milestone summary + lists.
- Uses identical computation logic to `/gaps` to avoid drift.

## 3) Query strategy

## Core helper views/CTEs
- `in_scope_hosts`: current in-scope hosts.
- `intent_observed_hosts(intent_set)`: host IPs observed in imports tagged with intent set.
- Anti-joins to derive `needs_*` queues.

## Performance indexes used
- `idx_host_in_scope`
- `idx_host_observation_project_ip`
- `idx_scan_import_intent_intent`
- `idx_port_host`
- `idx_port_status`

## 4) Backend implementation map

### DB layer
Create `internal/db/gaps.go`:
- `GetGapDashboard(projectID int64, opts GapOptions) (GapResponse, error)`
- `GetMilestoneQueues(projectID int64, opts GapOptions) (MilestoneQueueResponse, error)`

### Web layer
- Add routes in `internal/web/server.go`:
  - `GET /api/projects/{id}/gaps`
  - `GET /api/projects/{id}/queues/milestones`
- Add handlers in `internal/web/gap_handlers.go`.

## 5) Frontend plan

## Dashboard integration
- Add "Gap Dashboard" card/panel in `project.html`.
- In `dashboard.js`, fetch `/gaps` and render:
  - counts
  - preview lists
  - links to drill-down pages

## Drill-down links
- In-scope never scanned -> `hosts.html?id={id}&in_scope=true` (plus client-side filtering by list if advanced mode added).
- Open ports scanned/flagged -> `scan_results.html?id={id}&status=scanned` and `...&status=flagged`.
- Milestone lists -> dedicated lightweight `milestones.html` (optional) or section modal on dashboard.

## 6) Acceptance criteria
- All five list categories computed from data model, not inferred from current workflow counts.
- Milestone precedence exactly matches locked rules.
- Counts and list lengths are consistent.
- Works when imports exist but no intents (manual correction path still available).

## 7) Tests
- Unit tests:
  - `needs_ping_sweep` excludes hosts with top1k/all_tcp observations.
  - `needs_top_1k_tcp` excludes hosts with all_tcp observations.
  - `needs_all_tcp` only satisfied by all_tcp.
- Integration/API:
  - `/gaps` summary values match seeded fixture.
  - `/queues/milestones` mirrors milestone portions of `/gaps`.
- Regression:
  - Existing stats endpoint remains unchanged.
