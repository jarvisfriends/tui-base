#!/usr/bin/env bash
set -euo pipefail

echo "Running golangci-lint..."
golangci-lint run ./...

echo "Running tests with race detector and coverage..."
go test -race ./... -coverprofile=coverage.out

echo
echo "Coverage summary (functions):"
go tool cover -func=coverage.out | sed -n '$p'

echo
TOTAL_COVERAGE=$(go tool cover -func=coverage.out | awk '/total:/ {print $3}')
echo "Total coverage: ${TOTAL_COVERAGE}"

# Save a simple text file with the total percentage for CI parsing
go tool cover -func=coverage.out | awk '/total:/ {print $3}' > coverage.txt
echo "Wrote coverage.txt"
