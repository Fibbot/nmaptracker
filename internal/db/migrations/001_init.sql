BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS project (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scope_definition (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    definition TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS scan_import (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    import_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    hosts_found INTEGER DEFAULT 0,
    ports_found INTEGER DEFAULT 0,
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS host (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    ip_address TEXT NOT NULL,
    hostname TEXT,
    os_guess TEXT,
    in_scope BOOLEAN NOT NULL DEFAULT 1,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, ip_address),
    FOREIGN KEY(project_id) REFERENCES project(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS port (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id INTEGER NOT NULL,
    port_number INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    state TEXT NOT NULL,
    service TEXT,
    version TEXT,
    product TEXT,
    extra_info TEXT,
    work_status TEXT NOT NULL DEFAULT 'scanned',
    script_output TEXT,
    notes TEXT,
    last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(host_id, port_number, protocol),
    FOREIGN KEY(host_id) REFERENCES host(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_host_project ON host(project_id);
CREATE INDEX IF NOT EXISTS idx_host_ip ON host(ip_address);
CREATE INDEX IF NOT EXISTS idx_host_in_scope ON host(project_id, in_scope);
CREATE INDEX IF NOT EXISTS idx_port_host ON port(host_id);
CREATE INDEX IF NOT EXISTS idx_port_status ON port(work_status);
CREATE INDEX IF NOT EXISTS idx_port_number ON port(port_number);
CREATE INDEX IF NOT EXISTS idx_port_protocol ON port(protocol);
CREATE INDEX IF NOT EXISTS idx_scope_project ON scope_definition(project_id);

COMMIT;
