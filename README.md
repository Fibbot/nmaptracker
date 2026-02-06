# Nmap Tracker

Nmap Tracker is a lightweight, self-hosted tool for managing and visualizing Nmap scan results. It allows you to import Nmap XML reports, track host and port statuses over time, and manage scope through a unified web interface or CLI.

Vibecoded as hell testing out Antigravity/Codex.

## Features

*   **Project + Scan Ingestion**: Import Nmap XML (`-oX`) into per-project datasets with persisted scan history.
*   **Scope-Driven Workflow**: Manage in-scope/out-of-scope targeting with host/port workflow states (`scanned`, `flagged`, `in_progress`, `done`) and analyst notes.
*   **Import Intents + Coverage Matrix**: Tag scans by intent (ping/top-ports/full TCP/UDP/vuln) and visualize coverage with missing-host drilldowns.
*   **Import Delta Analysis**: Compare any two imports to surface net new/disappeared hosts, exposure changes, and service fingerprint drift.
*   **Expected Asset Baseline**: Track expected IPv4 IP/CIDR inventory and evaluate unseen expected assets or out-of-baseline observations.
*   **Service Campaign Queues**: Host-grouped SMB/LDAP/RDP/HTTP(S)/SSH queues with multi-select filters, per-host status summaries, and source import IDs.
*   **Queue Export Utilities**: Copy selected queue IPs to clipboard or export newline-delimited TXT host lists from the service queue page.
*   **Flexible Export + API**: Export project/host data via web endpoints (JSON/CSV/TXT) and CLI export (JSON/CSV).


<details>
<summary>UI Screens</summary>
<img width="1512" height="814" alt="ui - 1" src="https://github.com/user-attachments/assets/3a918b3a-34e9-49a6-8ebb-dba069f3ce9d" />
<p></p>
<img width="1512" height="539" alt="ui - 2" src="https://github.com/user-attachments/assets/c3e6cd7f-46ad-4c22-80c6-8e2d426e07e6" />
<p></p>
<img width="1511" height="810" alt="ui - 3" src="https://github.com/user-attachments/assets/50598b98-4879-4199-bbcb-8354036c06a6" />
<p></p>
</details>

## TL;DR (Web UI - Preferred)

1.  **Build**: `make build`
2.  **Serve**: `./nmap-tracker serve`
3.  **View**: Open `http://127.0.0.1:8080`

## TL;DR (CLI - If Masochistic)

1.  **Build**: `make build`
2.  **Create Project**: `./nmap-tracker projects create internal-audit`
3.  **Import Scan**: `./nmap-tracker import scan.xml --project internal-audit`
4.  **Export**: `./nmap-tracker export --project internal-audit --output internal-audit.json`

## Build without Make

If you don't have Make installed, you can build directly with Go:

```bash
go build ./cmd/nmap-tracker
```

## Build Instructions

### Prerequisites
*   Go (1.21 or later)
*   Make (optional, for convenience)

### Local Build
To build the binary for your current operating system:
```bash
make build
# Output binary: cmd/nmap-tracker/nmap-tracker (or just nmap-tracker depending on path)
```

### Cross-Platform Build
To build for all supported platforms (Linux, macOS, Windows; AMD64/ARM64):
```bash
make build-all
# Output binaries will be in the dist/ directory
```

## CLI Usage

The application uses a subcommand structure: `nmap-tracker <command> [flags]`

### 1. `projects`
Manage projects within the database.

*   **List Projects**:
    ```bash
    nmap-tracker projects list [--db <path>]
    ```
*   **Create Project**:
    ```bash
    nmap-tracker projects create <project-name> [--db <path>]
    ```

### 2. `import`
Import an Nmap XML scan file into a project.

```bash
nmap-tracker import <xml-file> --project <project-name> [--db <path>]
```
*   **Arguments**:
    *   `<xml-file>`: Path to the Nmap XML output.
*   **Flags**:
    *   `--project`: (Required) Name of the target project.
    *   `--db`: Path to SQLite DB (default: `nmap-tracker.db`).

### 3. `serve`
Start the web server to view and manage data.

```bash
nmap-tracker serve [--port <port>] [--db <path>]
```
*   **Flags**:
    *   `--port`: Port to listen on (default: `8080`).
    *   `--db`: Path to SQLite DB (default: `nmap-tracker.db`).

**Security Note:** The server binds to `127.0.0.1` only and includes a same-origin guard for browser requests. CLI/curl requests without an `Origin` header are still allowed.

### 4. `export`
Export project data to a file.

```bash
nmap-tracker export --project <project-name> --output <file> [--format <json|csv>] [--db <path>]
```
*   **Flags**:
    *   `--project`: (Required) Name of the source project.
    *   `--output`, `-o`: (Required) Path to the output file.
    *   `--format`: Output format, `json` or `csv` (default: `json`).
    *   `--db`: Path to SQLite DB.

## Examples

**1. Setting up a new engagement**
```bash
# Build the tool
make build

# Create a new project
./nmap-tracker projects create "External Pen Test 2024"

# Import the first scan
./nmap-tracker import initial_discovery.xml --project "External Pen Test 2024"
```

**2. Running the UI**
```bash
./nmap-tracker serve --port 9000
# Access at http://localhost:9000
```

**3. Exporting data for reporting**
```bash
./nmap-tracker export --project "External Pen Test 2024" --output results.csv --format csv
```

## Database
The application uses a local SQLite database (`nmap-tracker.db` by default). This file is automatically created if it doesn't exist when you run a command that requires DB access (like `make serve` or `projects create`).
