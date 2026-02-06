#!/usr/bin/env bash
set -e

# 1. Navigate from 'api/' directory up to Project Root
cd "$(dirname "$0")/.."

echo "Running API Integration Tests (Go)..."

# 2. Run the integration tests located in tests/ folder
# -v: Verbose output so you can see which endpoints are being hit
go test -v ./tests/...