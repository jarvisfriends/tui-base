#!/usr/bin/env bash
set -euo pipefail

# SKIP_LINT=1 skips the lint pass — CI sets it in the coverage job because the
# dedicated lint job has already run golangci-lint on the same commit.
if [[ "${SKIP_LINT:-0}" != "1" ]]; then
  echo "Running golangci-lint..."
  golangci-lint run ./...
fi

echo "Running tests with race detector and coverage..."
go test -race ./... -coverprofile=coverage.out

# Library scope: examples/, cmd/, tools/, and testutil/ are demo/wiring code
# exercised by hand (same exclusions as codecov.yml). The 90% gate applies to
# the library packages the test suite can meaningfully cover.
awk 'NR==1 || ($0 !~ /\/examples\// && $0 !~ /\/cmd\// && $0 !~ /\/tools\// && $0 !~ /\/testutil\//)' coverage.out > coverage.lib.out

echo
echo "Coverage summary (library scope):"
go tool cover -func=coverage.lib.out | sed -n '$p'

TOTAL_COVERAGE=$(go tool cover -func=coverage.lib.out | awk '/total:/ {print $3}')
echo "Total library-scope coverage: ${TOTAL_COVERAGE}"

# Save a simple text file with the total percentage for CI parsing
echo "${TOTAL_COVERAGE}" > coverage.txt
echo "Wrote coverage.txt"

THRESHOLD="${COVERAGE_THRESHOLD:-90.0}"
percent="${TOTAL_COVERAGE%\%}"
awk -v p="$percent" -v t="$THRESHOLD" 'BEGIN { if (p+0 < t+0) { printf "Coverage %s%% is below threshold %s%%\n", p, t; exit 1 } printf "Coverage %s%% meets threshold %s%%\n", p, t }'
