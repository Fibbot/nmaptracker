#!/bin/bash

if [ -z "$1" ] || [ -z "$2" ]; then
    echo "Usage: $0 <path_to_xml> <project_name>"
    exit 1
fi

XML_PATH=$1
PROJECT_NAME=$2
TEMP_BIN="./tmp_nmap_tracker"

echo "Building temporary nmap-tracker binary..."
go build -o $TEMP_BIN ./cmd/nmap-tracker

if [ $? -ne 0 ]; then
    echo "Build failed."
    exit 1
fi

echo "Importing scan from $XML_PATH into project $PROJECT_NAME..."
$TEMP_BIN import "$XML_PATH" --project "$PROJECT_NAME"

echo "Cleaning up..."
rm $TEMP_BIN

echo "Done."
