# NmapTracker - Project Specification

## What This Is

NmapTracker is a local-first web application for penetration testers to track nmap scan coverage and assessment progress across large network engagements. It solves the "did I actually hit that /24 with full ports?" problem by providing a single pane of glass for scan data with per-port workflow tracking.

This is **not** a findings tracker or vulnerability database. It tracks coverage and work status only.

## Core Problem Statement

During network penetration tests, especially on large scopes:
- Multiple scan types run at different times (quick TCP, full TCP, UDP, service detection)
- Easy to lose track of what's been scanned vs. what's been assessed
- Partial scans may cover overlapping ranges
- Need to know: "What still needs attention?" and "What can I come back to if I have time?"

## Technical Stack

- **Language**: Go (for cross-platform single-binary distribution)
- **Database**: SQLite (embedded, no external dependencies)
- **Web UI**: Embedded web server serving server-rendered HTML via `templ`
- **Build targets**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64

## Core Functionality

### 1. Project Management
- Create/delete projects by name
- Each project has isolated scope definitions, hosts, and ports
- Projects are the top-level organizational unit

### 2. Scope Definition
- Define in-scope ranges using:
  - CIDR notation (`10.0.0.0/24`)
  - IP ranges (`10.0.0.1-50`)
  - Individual IPs (`10.0.0.100`)
- Define exclusions (same formats, takes precedence over includes)
- Hosts are automatically flagged as in-scope or out-of-scope on import
- Out-of-scope hosts are visually distinguished but not rejected
- **Current implementation note**: there is no UI or CLI for editing scope definitions yet. Imports run with `includeAllByDefault=true` (so with no scope definitions, all hosts are in scope). Scope definitions exist in the DB schema and exports.

### 3. Scan Import
- Parse nmap XML output (from `-oX` or `-oA`)
- Greppable format (`.gnmap`) is **not implemented yet** (placeholder exists)
- Current import path is CLI-only (no web UI upload yet)
- **Merge behavior**:
  - New hosts are added
  - New ports on existing hosts are added
  - Existing ports get service/version/product/extra_info/script_output updated only when the new scan provides non-empty values
  - Work status and notes are preserved on existing ports
  - Ports are **never auto-removed** by subsequent scans (prevents partial scan data loss)
  - Each `(host, port, protocol)` tuple is tracked independently
- Track import history (filename, timestamp, stats) for audit trail

### 4. Port Tracking Data
For each observed port (open/closed/filtered/etc.), store:
- Port number and protocol (TCP/UDP)
- nmap state (open/closed/filtered)
- Service name, version, product info (from `-sV`)
- NSE script output as blob (from `-sC` or custom scripts)
- Last seen timestamp
- Notes field (user-editable)

### 5. Workflow State Machine
Each **open port** has a work status:

```
[scanned] → [flagged] → [in_progress] → [done]
                ↓                          ↓
          [parking_lot] ←←←←←←←←←←←←←←←←←←←┘
```

- `scanned` - Imported but not yet assessed (default state)
- `flagged` - Manually marked as having attack surface worth investigating
- `in_progress` - Currently being worked on
- `done` - Assessment complete
- `parking_lot` - Noted for later if time permits

State transitions are manual. No automated "this looks interesting" logic.
Implementation detail: work status is stored for all ports, but the UI and bulk updates only allow changes on `state=open`.

### 6. Views and Filtering

**Project Dashboard**
- Total hosts (in-scope vs out-of-scope breakdown)
- Open-port status rollup (counts by work_status, open ports only)
- Progress percentage (done / (flagged + in_progress + done + parking_lot), open ports only)

**Host List View**
- Sortable/filterable table
- Columns: IP, hostname, port count, status summary, in-scope indicator
- Filters: by subnet/CIDR, by work_status presence (open ports), by in-scope
- Click through to host detail
- Pagination (default page size 50; max 500)
- Status summary is based on open ports only; port count includes all ports

**Host Detail View**
- Host metadata (IP, hostname, OS guess, notes)
- Port table with all tracked ports
- Inline status dropdowns for open ports
- Per-port notes (open ports)
- Service/version/product/extra info is shown inline; script output is stored but not displayed in the UI
- Port table defaults to `state=open` only; can filter by `state` query params

**Bulk Operations**
- "Mark all open ports on this host as [status]"
- "Mark all open ports with [port number] across project as [status]"
- "Mark all open ports for hosts in the current host list view as [status]"

### 7. Export
- Export to JSON (full project data: project, scope definitions, scan imports, hosts, ports)
- Export to CSV (flattened port list with host + project info)
- Web UI supports export by project and by host; CLI exports project data only

## Architecture Guidelines

### Directory Structure
```
nmap-tracker/
├── cmd/
│   └── nmap-tracker/
│       └── main.go           # Entry point
├── internal/
│   ├── db/
│   │   ├── db.go             # SQLite connection, migrations
│   │   ├── models.go         # Struct definitions
│   │   ├── project.go        # Project CRUD
│   │   ├── host.go           # Host CRUD
│   │   ├── port.go           # Port CRUD + bulk ops
│   │   ├── scope.go          # Scope definition CRUD
│   │   ├── dashboard.go      # Dashboard stats
│   │   ├── host_list.go      # Host list aggregation
│   │   └── scan_import.go    # Scan import records
│   ├── importer/
│   │   ├── importer.go       # Import orchestration
│   │   ├── xml.go            # nmap XML parser
│   │   └── gnmap.go          # greppable format parser
│   ├── scope/
│   │   └── matcher.go        # IP-in-scope evaluation
│   ├── web/
│   │   ├── server.go         # HTTP server setup
│   │   ├── handlers.go       # Route handlers
│   │   └── templates.go      # templ-rendered HTML
│   └── export/
│       ├── json.go
│       └── csv.go
├── agent_docs/
│   ├── ERD.md
│   └── SPEC.md               # This file
├── go.mod
├── go.sum
├── Makefile                  # Build targets for all platforms
└── README.md
```

### Key Libraries (Suggested)
- `github.com/mattn/go-sqlite3` or `modernc.org/sqlite` (pure Go, easier cross-compile)
- `github.com/go-chi/chi/v5` - routing
- `github.com/a-h/templ` - type-safe templates
- XML parsing via stdlib `encoding/xml`
- IP/CIDR handling via stdlib `net` or `netip`

### Database
- SQLite with WAL mode for better concurrency
- Migrations embedded in binary
- Schema defined in `internal/db/migrations/`

### Web UI
- Server-rendered HTML via `templ`
- Minimal JS, progressive enhancement
- Mobile-responsive (used on laptops in the field)

## Non-Goals (Explicitly Out of Scope)

1. **Finding tracking** - No vulnerability descriptions, severity ratings, or remediation notes
2. **Automated analysis** - No "this version is vulnerable" lookups
3. **Network scanning** - This tool does not run nmap, only consumes its output
4. **Multi-user auth** - Single-user local tool (future stretch goal consideration)
5. **Cloud/hosted mode** - Local-first, always

## CLI Interface

```bash
# Start the web server
nmap-tracker serve [--port 8080] [--db ./nmap-tracker.db]

# Import scan (XML only)
nmap-tracker import --project "Client ABC" scan-results.xml

# Export project
nmap-tracker export --project "Client ABC" --format json -o export.json

# List projects
nmap-tracker projects list

# Create project
nmap-tracker projects create "Client ABC"
```

## Development Notes

### Build Commands
```bash
# Local development
go run ./cmd/nmap-tracker serve

# Build all platforms
make build-all

# Run tests
go test ./...
```

### Testing Priorities
1. XML/gnmap parsing correctness
2. Merge logic (subsequent scans don't clobber data)
3. Scope matching (CIDR, ranges, exclusions)
4. Bulk operations atomicity

## Implementation Notes
- SQLite driver: Use `modernc.org/sqlite` for a pure Go, cross-compile-friendly build; only switch to `mattn/go-sqlite3` if we later accept CGO for performance reasons.
- UI approach: Server-rendered HTML with `templ`; no SPA.
- Work status scope: Work status is stored on all ports, but UI/handlers only expose state transitions for ports with `state=open`; closed/filtered ports stay at their existing status (default `scanned`).
- Additive imports: Imports never delete ports; rely on `last_seen` to surface “not seen recently” without removal.
- Scope defaults: CLI import builds a matcher with `includeAllByDefault=true`, so in the absence of scope definitions all hosts are treated as in scope.

## Future Considerations (Stretch Goals)

These are explicitly **not** in v1 but worth keeping in mind architecturally:

1. **Multi-user support** - Would require auth layer, user_id on mutations, possibly switching to PostgreSQL
2. **Real-time sync** - WebSocket updates if multiple browser tabs
3. **Scan diffing** - "What changed between scan A and scan B?"
4. **Target generation** - Export "remaining unscanned" as nmap target list
