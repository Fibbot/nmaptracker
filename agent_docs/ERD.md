# Entity Relationship Diagram

## Overview

This document describes the data model for the nmap scan tracking application. The model is designed around project-based assessments with protocol-aware port tracking and per-port workflow state management.

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                  PROJECT                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│ PK  id              INTEGER                                                  │
│     name            TEXT NOT NULL                                            │
│     created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│     updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ 1:N
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                SCOPE_DEFINITION                              │
├─────────────────────────────────────────────────────────────────────────────┤
│ PK  id              INTEGER                                                  │
│ FK  project_id      INTEGER NOT NULL → PROJECT(id) ON DELETE CASCADE         │
│     definition      TEXT NOT NULL  -- CIDR, range, or single IP              │
│     type            TEXT NOT NULL  -- 'include' | 'exclude'                  │
│     created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                  PROJECT                                     │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ 1:N
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                SCAN_IMPORT                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│ PK  id              INTEGER                                                  │
│ FK  project_id      INTEGER NOT NULL → PROJECT(id) ON DELETE CASCADE         │
│     filename        TEXT NOT NULL                                            │
│     import_time     TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│     hosts_found     INTEGER DEFAULT 0                                        │
│     ports_found     INTEGER DEFAULT 0                                        │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                  PROJECT                                     │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ 1:N
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                    HOST                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│ PK  id              INTEGER                                                  │
│ FK  project_id      INTEGER NOT NULL → PROJECT(id) ON DELETE CASCADE         │
│     ip_address      TEXT NOT NULL                                            │
│     hostname        TEXT  -- from reverse DNS or nmap detection              │
│     os_guess        TEXT  -- nmap OS detection if available                  │
│     in_scope        BOOLEAN NOT NULL DEFAULT TRUE  -- derived from scope     │
│     notes           TEXT                                                     │
│     created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│     updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│                                                                              │
│     UNIQUE(project_id, ip_address)                                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       │ 1:N
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                    PORT                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│ PK  id              INTEGER                                                  │
│ FK  host_id         INTEGER NOT NULL → HOST(id) ON DELETE CASCADE            │
│     port_number     INTEGER NOT NULL  -- 1-65535                             │
│     protocol        TEXT NOT NULL     -- 'tcp' | 'udp'                       │
│     state           TEXT NOT NULL     -- 'open' | 'closed' | 'filtered'      │
│     service         TEXT              -- e.g., 'http', 'ssh'                 │
│     version         TEXT              -- full version string from -sV        │
│     product         TEXT              -- product name from -sV               │
│     extra_info      TEXT              -- additional service info             │
│     work_status     TEXT NOT NULL DEFAULT 'scanned'                          │
│                     -- 'scanned' | 'flagged' | 'in_progress'                 │
│                     -- | 'done' | 'parking_lot'                              │
│     script_output   TEXT              -- blob of NSE script output           │
│     notes           TEXT                                                     │
│     last_seen       TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│     created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│     updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP                      │
│                                                                              │
│     UNIQUE(host_id, port_number, protocol)                                   │
└─────────────────────────────────────────────────────────────────────────────┘

```

## Relationships Summary

| Parent          | Child            | Cardinality | ON DELETE |
|-----------------|------------------|-------------|-----------|
| PROJECT         | SCOPE_DEFINITION | 1:N         | CASCADE   |
| PROJECT         | SCAN_IMPORT      | 1:N         | CASCADE   |
| PROJECT         | HOST             | 1:N         | CASCADE   |
| HOST            | PORT             | 1:N         | CASCADE   |

## Enumerations

### scope_definition.type
- `include` - IP/range/CIDR is in scope
- `exclude` - IP/range/CIDR is explicitly out of scope (takes precedence)

### port.state (nmap states)
Stored as reported by nmap; commonly:
- `open` - Port is accepting connections
- `closed` - Port is accessible but no service listening
- `filtered` - Firewall/filtering preventing determination

### port.work_status (workflow states)
- `scanned` - Initial state after import, no assessment yet
- `flagged` - Manually marked as having attack surface worth investigating
- `in_progress` - Currently being investigated
- `done` - Assessment complete for this port
- `parking_lot` - Noted for later if time permits

## Indexes

```sql
-- Performance indexes for common query patterns
CREATE INDEX idx_host_project ON host(project_id);
CREATE INDEX idx_host_ip ON host(ip_address);
CREATE INDEX idx_host_in_scope ON host(project_id, in_scope);
CREATE INDEX idx_port_host ON port(host_id);
CREATE INDEX idx_port_status ON port(work_status);
CREATE INDEX idx_port_number ON port(port_number);
CREATE INDEX idx_port_protocol ON port(protocol);
CREATE INDEX idx_scope_project ON scope_definition(project_id);
```

## Notes on Design Decisions

1. **Protocol-aware port tracking**: `(host_id, port_number, protocol)` is the unique key, allowing TCP and UDP to be tracked independently for the same port number.

2. **Additive scan imports**: Subsequent scans add/update ports but never remove them. This prevents partial scans from destroying earlier comprehensive scan data.

3. **Scope as separate entity**: Allows complex scope definitions (multiple CIDRs, exclusions) without string parsing on every query. `in_scope` on HOST is denormalized for query performance but derived from SCOPE_DEFINITION on import/update.

4. **Work status on PORT, not HOST**: Enables granular tracking of assessment progress per-service rather than per-machine.

5. **Script output as blob**: NSE output can be extensive and varied; stored as-is for reference without attempting to parse structure.
