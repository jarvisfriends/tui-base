#!/usr/bin/env bash
set -euo pipefail

# FUZZTIME can be overridden (e.g. FUZZTIME=60s bash tools/fuzz.sh); CI uses
# a short smoke duration per target (CI-3).
FUZZTIME="${FUZZTIME:-10s}"

echo "Running fuzzers (${FUZZTIME} each)..."
go test ./envpath -run '^$' -fuzz='^FuzzCollapseExpand$' -fuzztime="${FUZZTIME}"

echo "Fuzz runs completed (timed)."
