#!/usr/bin/env bash
set -euo pipefail

echo "Running fuzzers for navigation and debug (10s each)..."
go test ./navigation -run '^$' -fuzz=Fuzz -fuzztime=10s
go test ./pages/debug -run '^$' -fuzz=Fuzz -fuzztime=10s

echo "Fuzz runs completed (timed)."
