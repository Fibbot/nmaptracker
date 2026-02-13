BEGIN TRANSACTION;

ALTER TABLE scan_import ADD COLUMN nmap_args TEXT NOT NULL DEFAULT '';
ALTER TABLE scan_import ADD COLUMN scanner_label TEXT NOT NULL DEFAULT '';
ALTER TABLE scan_import ADD COLUMN source_ip TEXT;
ALTER TABLE scan_import ADD COLUMN source_port INTEGER CHECK(source_port IS NULL OR (source_port BETWEEN 1 AND 65535));
ALTER TABLE scan_import ADD COLUMN source_port_raw TEXT;

COMMIT;
