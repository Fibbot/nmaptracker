# Expected Asset Baseline - Implementation Spec

## Goal
Allow users to define expected assets/subnets and evaluate:
- expected but unseen
- seen but out-of-scope (relative to expected baseline)

## Dependencies
- `expected_asset_baseline` table from `features/01`.
- Current `host` table for observed assets.

## 1) Baseline ingest rules

## Accepted definitions
- IPv4 single IP (e.g., `10.0.5.10`)
- IPv4 CIDR (e.g., `10.0.5.0/24`)

## Rejected definitions
- Invalid IP/CIDR syntax.
- IPv6 definitions (current release scope is IPv4 only).
- CIDR broader than `/16` (safety cap; max expansion 65,536 addresses).

## Type inference
- `type='ip'` for single addresses.
- `type='cidr'` for prefixes.

## Dedupe
- DB unique constraint `(project_id, definition)`.
- API treats duplicates as no-op additions.

## 2) Evaluation semantics (locked)

Let:
- `E` = expanded expected IPv4 set from baseline definitions.
- `S` = observed host IPs in `host` for project.

Outputs:
- **expected_but_unseen** = `E - S`
- **seen_but_out_of_scope** = hosts in `S` that are not in `E`

Additional classification for operator clarity:
- `seen_but_out_of_scope_and_marked_in_scope`: subset where `host.in_scope=1`
- `seen_but_out_of_scope_and_marked_out_scope`: subset where `host.in_scope=0`

## Expansion algorithm
- Expand each CIDR to concrete IPv4 addresses in-memory.
- Because `/16` max is enforced, worst-case expansion is bounded.
- Sort outputs by numeric IP ascending.

## 3) API contracts

### `GET /api/projects/{id}/baseline`
Response:
```json
{
  "items": [
    {"id": 10, "project_id": 3, "definition": "10.0.0.0/24", "type": "cidr", "created_at": "2026-02-01T10:00:00Z"}
  ],
  "total": 1
}
```

### `POST /api/projects/{id}/baseline`
Request:
```json
{
  "definitions": ["10.0.0.0/24", "10.0.1.5"]
}
```
Response:
```json
{
  "added": 2,
  "items": [
    {"id": 10, "definition": "10.0.0.0/24", "type": "cidr"},
    {"id": 11, "definition": "10.0.1.5", "type": "ip"}
  ]
}
```

### `DELETE /api/projects/{id}/baseline/{baselineID}`
Response: `204 No Content`

### `GET /api/projects/{id}/baseline/evaluate`
Response:
```json
{
  "generated_at": "2026-02-04T19:25:00Z",
  "project_id": 3,
  "summary": {
    "expected_total": 512,
    "observed_total": 498,
    "expected_but_unseen": 27,
    "seen_but_out_of_scope": 13
  },
  "lists": {
    "expected_but_unseen": ["10.0.1.44"],
    "seen_but_out_of_scope": [
      {"host_id": 88, "ip_address": "10.0.8.9", "hostname": "", "in_scope": false}
    ]
  }
}
```

## 4) Backend implementation map

### DB layer
Create `internal/db/baseline.go`:
- CRUD helpers for baseline definitions.
- Evaluation helper that:
  - loads baseline definitions,
  - expands expected set,
  - loads observed hosts,
  - computes set diffs.

### Web layer
Create `internal/web/baseline_handlers.go`:
- `apiListBaseline`
- `apiAddBaseline`
- `apiDeleteBaseline`
- `apiEvaluateBaseline`

Register routes in `internal/web/server.go`.

## 5) Frontend plan

## Dashboard panel
Add "Expected Asset Baseline" section to `project.html`:
- textarea input (one definition per line)
- current baseline list with delete action
- evaluate button
- result summary and two lists

Update `dashboard.js`:
- `loadBaseline()`, `addBaseline()`, `deleteBaseline()`, `evaluateBaseline()`.

## 6) Acceptance criteria
- User can add/list/delete baseline definitions.
- Evaluation output exactly reflects expected-vs-observed set math.
- Large CIDR rejection returns clear 400 error.
- No duplicate baseline rows are created.

## 7) Tests
- Unit:
  - type inference IP vs CIDR.
  - set-diff correctness for expected/unseen and seen/out-of-scope.
  - CIDR size validation.
- API:
  - add/list/delete/evaluate response contracts.
  - invalid definitions and oversized CIDR errors.
- UI smoke:
  - baseline section add/remove/evaluate works with mocked data.
