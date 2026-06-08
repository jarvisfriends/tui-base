#!/usr/bin/env bash
set -euo pipefail

echo "Running tests with race detector and coverage..."
go test -race ./... -coverprofile=coverage.out

echo
echo "Coverage summary (functions):"
go tool cover -func=coverage.out | sed -n '$p'

echo
echo "Total coverage:" $(go tool cover -func=coverage.out | awk '/total:/ {print $3}')

# Save a simple text file with the total percentage for CI parsing
go tool cover -func=coverage.out | awk '/total:/ {print $3}' > coverage.txt
echo "Wrote coverage.txt"
