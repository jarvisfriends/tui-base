#!/usr/bin/env bash
set -euo pipefail

# SKIP_LINT=1 skips the lint pass — CI sets it in the coverage job because the
# dedicated lint job has already run golangci-lint on the same commit (CI-7).
if [[ "${SKIP_LINT:-0}" != "1" ]]; then
  echo "Running golangci-lint..."
  golangci-lint run ./...
fi

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
