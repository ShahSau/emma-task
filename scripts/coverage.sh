#!/usr/bin/env bash
set -e

echo "Running all tests with coverage..."

# -coverpkg=./... : Ensures integration tests in 'tests/' count towards coverage of 'users/', 'articles/' etc.
# ./... : Runs tests in all subdirectories (unit and integration)
go test -v -covermode=atomic -coverprofile=coverage.out -coverpkg=./... ./...

echo ""
echo "Coverage report generated: coverage.out"
go tool cover -func=coverage.out