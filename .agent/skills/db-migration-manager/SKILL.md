---
name: Database Migration Manager
description: Automates the creation of timestamped SQL migration files.
---

# Database Migration Manager

This skill simplifies the process of creating new database migrations. It ensures that migration files are named correctly with a timestamp prefix, which is required by the `internal/db/db.go` migration logic.

## Usage

To create a new migration, run the `create_migration.sh` script with a descriptive name for the migration.

```bash
./.agent/skills/db-migration-manager/create_migration.sh <migration_name>
```

## Example

```bash
./.agent/skills/db-migration-manager/create_migration.sh add_users_table
```

This will create a file named `YYYYMMDDHHMMSS_add_users_table.sql` in the `internal/db/migrations` directory.
