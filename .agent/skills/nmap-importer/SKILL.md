---
name: Nmap Scan Importer
description: Faciliates the dev loop of importing a scan XML file.
---

# Nmap Scan Importer

This skill simplifies the process of testing the `nmap-tracker` import functionality during development. It builds a temporary binary and imports a specified XML file into a project.

## Usage

```bash
./.agent/skills/nmap-importer/import_scan.sh <path_to_xml> <project_name>
```

## Example

```bash
./.agent/skills/nmap-importer/import_scan.sh sampleNmap.xml my_project
```

This will:
1. Build the CLI to a temporary location.
2. Run the import command with the provided arguments.
3. Clean up the temporary binary.
