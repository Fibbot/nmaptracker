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
- **Web UI**: Embedded web server serving a local UI (htmx + templ, or embedded SPA)
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
- Out-of-scope hosts are visually distinguished but not rejected (able to disable this functionality as i can see some people not caring)

### 3. Scan Import
- Parse nmap XML output (from `-oX` or `-oA`)
- Support greppable format (`.gnmap`) as secondary option
- **Merge behavior**:
  - New hosts are added
  - New ports on existing hosts are added
  - Existing ports get service/version data updated if new scan has richer info
  - Ports are **never auto-removed** by subsequent scans (prevents partial scan data loss)
  - Each `(host, port, protocol)` tuple is tracked independently
- Track import history (filename, timestamp, stats) for audit trail

### 4. Port Tracking Data
For each open port, store:
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

### 6. Views and Filtering

**Project Dashboard**
- Total hosts (in-scope vs out-of-scope breakdown)
- Port status rollup (counts by work_status)
- Progress percentage (done / total flagged)
- Subnet breakdown with coverage stats

**Host List View**
- Sortable/filterable table
- Columns: IP, hostname, port count, status summary, in-scope indicator
- Filters: by subnet/CIDR, by work_status presence, by in-scope
- Click through to host detail

**Host Detail View**
- Host metadata (IP, hostname, OS guess, notes)
- Port table with all tracked ports
- Inline status toggles (click to cycle work_status)
- Expandable service/version details
- Expandable script output (collapsed by default)
- Per-port notes

**Bulk Operations**
- "Mark all ports on this host as [status]"
- "Mark all [port number] across project as [status]"
- "Mark all filtered by current view as [status]"

### 7. Export
- Export to JSON (full project data)
- Export to CSV (flattened port list with host info)

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
│   │   └── scope.go          # Scope definition logic
│   ├── importer/
│   │   ├── importer.go       # Import orchestration
│   │   ├── xml.go            # nmap XML parser
│   │   └── gnmap.go          # greppable format parser
│   ├── scope/
│   │   └── matcher.go        # IP-in-scope evaluation
│   ├── web/
│   │   ├── server.go         # HTTP server setup
│   │   ├── handlers.go       # Route handlers
│   │   ├── templates/        # templ or html templates
│   │   └── static/           # CSS, JS if needed
│   └── export/
│       ├── json.go
│       └── csv.go
├── docs/
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
- `github.com/a-h/templ` - type-safe templates (or standard html/template)
- XML parsing via stdlib `encoding/xml`
- IP/CIDR handling via stdlib `net` or `netip`

### Database
- SQLite with WAL mode for better concurrency
- Migrations embedded in binary
- Schema defined in `internal/db/migrations/`

### Web UI
- Server-rendered HTML preferred (simpler, lighter)
- htmx for dynamic updates without full SPA complexity
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

# Import scan (can also be done via web UI)
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
- UI approach: Server-rendered HTML with `templ` plus htmx for progressive enhancements; no SPA.
- Work status scope: Work status is stored on all ports, but UI/handlers only expose state transitions for ports with `state=open`; closed/filtered ports stay at the default `scanned` status and are excluded from workflow toggles.
- Additive imports: Imports never delete ports; rely on `last_seen` to surface “not seen recently” without removal.

## Future Considerations (Stretch Goals)

These are explicitly **not** in v1 but worth keeping in mind architecturally:

1. **Multi-user support** - Would require auth layer, user_id on mutations, possibly switching to PostgreSQL
2. **Real-time sync** - WebSocket updates if multiple browser tabs
3. **Scan diffing** - "What changed between scan A and scan B?"
4. **Target generation** - Export "remaining unscanned" as nmap target list
