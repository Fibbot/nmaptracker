#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: $0 <migration_name>"
    exit 1
fi

MIGRATION_NAME=$1
TIMESTAMP=$(date +%Y%m%d%H%M%S)
FILENAME="${TIMESTAMP}_${MIGRATION_NAME}.sql"
MIGRATIONS_DIR="internal/db/migrations"

# Ensure migrations directory exists
mkdir -p "$MIGRATIONS_DIR"

FILEPATH="$MIGRATIONS_DIR/$FILENAME"

touch "$FILEPATH"
echo "-- Migration: $MIGRATION_NAME" > "$FILEPATH"
echo "-- Created at: $TIMESTAMP" >> "$FILEPATH"
echo "" >> "$FILEPATH"
echo "-- Write your SQL up migration here" >> "$FILEPATH"

echo "Created migration file: $FILEPATH"
