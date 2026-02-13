# Test Plan - Source Metadata Tracking

Manual testing checklist for source IP/source port tracking on scan imports.

## Pre-Test Setup
1. Build binary: `make build`
2. Start server: `./nmap-tracker serve --port 8080 --db nmap-tracker.db`
3. Create or open a project in the web UI (`http://127.0.0.1:8080`).

## Fix 1: Import-level source metadata persistence + API exposure
1. Import an XML that contains `nmaprun args` with `-S` and `--source-port`.
2. Open project dashboard -> **Import Intents** table.
3. Confirm row shows:
   - Source IP from args
   - Source port from args
   - Nmap args preview with truncation and expandable full text
4. Call `GET /api/projects/{id}/imports` and confirm each import includes:
   - `nmap_args`
   - `scanner_label`
   - `source_ip`
   - `source_port`
   - `source_port_raw`

### Test notes
<empty for user>
---

## Fix 2: Manual fallback metadata via Web Import
1. In dashboard import section, fill optional fields:
   - Scanner Label
   - Source IP (IPv4)
   - Source Port
2. Import an XML with args that do **not** contain `-S`/`-g`.
3. Confirm table/API show manual values as canonical metadata.
4. Re-test with invalid manual values:
   - Source IP: `not-an-ip`
   - Source Port: `70000`
5. Confirm import request is rejected with 400-level validation feedback.

### Test notes
<empty for user>
---

## Fix 3: CLI fallback metadata flags
1. Run import with new flags:
   - `--scanner-label`
   - `--source-ip`
   - `--source-port`
2. Confirm resulting import row/API response contains provided metadata.
3. Re-run with invalid `--source-ip` or out-of-range `--source-port`.
4. Confirm CLI exits non-zero and prints validation error.

### Test notes
<empty for user>
---

## Fix 4: Source port fallback display semantics
1. Import scan with no source-port flag and no manual source-port.
2. Confirm UI shows: `default (no source port spoofing)`.
3. Import scan where args contain invalid source-port token (e.g. `--source-port banana`) and no manual source-port.
4. Confirm UI shows `unparsed: banana`.
5. (Optional) Re-test with manual source-port provided; confirm UI shows numeric canonical source port and API still preserves `source_port_raw`.

### Test notes
<empty for user>
