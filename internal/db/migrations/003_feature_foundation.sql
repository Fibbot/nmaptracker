BEGIN TRANSACTION;

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

COMMIT;
