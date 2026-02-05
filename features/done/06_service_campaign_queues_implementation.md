# Service-Campaign Queues - Implementation Spec

## Goal
Provide auditable quick-pivot queues for protocol campaigns and expose milestone queues from the same workflow surface.

## Dependencies
- Current `host` + `port` tables.
- Gap/milestone endpoint from `features/03`.
- Import metadata (for audit context) from `features/01`.

## 1) Campaign definitions (locked)

Campaign matching uses deterministic OR rules on `(port_number, protocol, service)` and includes only in-scope hosts.

## Common filters
- `host.project_id = ?`
- `host.in_scope = 1`
- `port.state IN ('open','open|filtered')`

## SMB
Match when any:
- `(port_number=445 AND protocol='tcp')`
- `service` contains `smb` or `microsoft-ds` or `netbios-ssn`

## LDAP
Match when any:
- `(389/tcp)`, `(636/tcp)`, `(3268/tcp)`, `(3269/tcp)`
- `service` contains `ldap` or `ldaps`

## RDP
Match when any:
- `(3389/tcp)`
- `service` contains `rdp` or `ms-wbt-server`

## HTTP
Match when any:
- Ports: `80,81,443,8000,8080,8081,8443,8888` with `protocol='tcp'`
- `service` contains `http`, `https`, or `http-proxy`

## 2) Queue outputs

Each queue item (host-level grouping):
- `host_id`, `ip_address`, `hostname`
- `matching_ports`: array of `{port_id, port_number, protocol, state, service, product, version, work_status, last_seen}`
- `status_summary`: counts by `work_status` within matching ports
- `latest_seen`: max `last_seen` in matching ports

Campaign response envelope includes:
- `generated_at`
- `project_id`
- `campaign`
- `filters_applied`
- `source_import_ids` (distinct import IDs where host was observed, if available from `host_observation`)

## 3) API contracts

### `GET /api/projects/{id}/queues/services`

#### Query params
- `campaign` required; one of `smb|ldap|rdp|http`
- `page`, `page_size` default `1`, `50`, max `500`

#### Response
```json
{
  "generated_at": "2026-02-04T19:30:00Z",
  "project_id": 3,
  "campaign": "smb",
  "filters_applied": {"states": ["open","open|filtered"]},
  "total_hosts": 12,
  "items": [
    {
      "host_id": 55,
      "ip_address": "10.0.2.20",
      "hostname": "filesrv01",
      "latest_seen": "2026-02-02T22:11:00Z",
      "status_summary": {"scanned": 1, "flagged": 1, "in_progress": 0, "done": 0, "parking_lot": 0},
      "matching_ports": [
        {
          "port_id": 800,
          "port_number": 445,
          "protocol": "tcp",
          "state": "open",
          "service": "microsoft-ds",
          "product": "Samba",
          "version": "4.13",
          "work_status": "flagged",
          "last_seen": "2026-02-02T22:11:00Z"
        }
      ]
    }
  ],
  "source_import_ids": [70, 72]
}
```

### `GET /api/projects/{id}/queues/milestones`
- Reuse implementation from feature 03.
- Endpoint appears in this feature for adjacent workflow UX.

## 4) Backend implementation map

### DB layer
Create `internal/db/service_queues.go`:
- `ListServiceCampaignQueue(projectID int64, campaign string, limit, offset int) (items []ServiceCampaignHost, total int, sourceImportIDs []int64, err error)`

Implementation details:
- Use campaign-specific SQL predicates from a centralized matcher map.
- Group rows by host in Go after query.
- Compute status summary per host from matching rows.

### Web layer
Create `internal/web/queue_handlers.go`:
- `apiListServiceQueue`
- route registration in `internal/web/server.go`:
  - `GET /api/projects/{id}/queues/services`

Validation:
- reject unknown campaign with 400.

## 5) Frontend plan

## Dashboard quick pivots
In `project.html` add "Service Campaign Queues" card with buttons:
- SMB
- LDAP
- RDP
- HTTP
- Milestones (adjacent queue view)

## New page
- `internal/web/frontend/service_queues.html`
- `internal/web/frontend/js/service_queues.js`

UI behavior:
- Campaign selector chips.
- Host-grouped table with expandable matching ports.
- Status summary badges.
- Row link to `host.html?id={projectId}&hostId={hostID}`.

## Milestone adjacency
- Add tab/secondary section that calls `/queues/milestones` and shows:
  - needs ping
  - needs top1k
  - needs all_tcp

## 6) Acceptance criteria
- Campaign matching is deterministic and documented.
- Queue includes only in-scope hosts and open/open|filtered ports.
- Milestone queue accessible from same workflow area.
- Audit metadata present in API response.

## 7) Tests
- Unit:
  - campaign predicate matching per service and per port-based fallback.
  - host grouping and status summary aggregation.
- API:
  - unknown campaign validation.
  - paginated queue shape and totals.
- Integration:
  - milestone endpoint visible and functional from queue UI.
- UI smoke:
  - campaign switching refreshes data and drill-down links remain correct.
