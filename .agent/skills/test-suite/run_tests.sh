#!/bin/bash

TARGETS=${1:-./...}

echo "Running tests for: $TARGETS"
go test -race -v $TARGETS
