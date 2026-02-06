#!/bin/bash

# Check if all Go files are properly formatted
unformatted=$(gofmt -l .)

if [ -n "$unformatted" ]; then
    echo "ERROR: The following files are not formatted:"
    echo "$unformatted"
    echo ""
    echo "Please run: go fmt ./..."
    exit 1
else
    echo "Success: All code is formatted."
    exit 0
fi