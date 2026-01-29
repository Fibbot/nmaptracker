#!/bin/bash

# Check if templ is installed
if ! command -v templ &> /dev/null; then
    echo "templ could not be found. Installing..."
    go install github.com/a-h/templ/cmd/templ@latest
fi


# Try to find templ in PATH
if command -v templ &> /dev/null; then
    TEMPL_CMD="templ"
else
    # Check if it's in GOPATH/bin
    GOBIN=$(go env GOPATH)/bin
    if [ -x "$GOBIN/templ" ]; then
        TEMPL_CMD="$GOBIN/templ"
    else
        echo "templ not found in PATH or GOPATH/bin. Please ensure it is installed and in your PATH."
        exit 1
    fi
fi

echo "Generating Templ components using $TEMPL_CMD..."
$TEMPL_CMD generate
