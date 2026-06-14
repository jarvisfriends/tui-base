#!/usr/bin/env bash
set -euo pipefail

# race_test.sh — run the Go test suite with the race detector enabled.
#
# Forces CGO_ENABLED=1 (required by -race) and forwards any extra arguments to
# `go test`. With no arguments it tests the whole module (./...).
#
# Usage:
#   tools/race_test.sh                    # go test -race ./...
#   tools/race_test.sh ./pages/...        # scope to packages
#   tools/race_test.sh -run TestFoo ./... # forward flags to go test
#
# If the C toolchain is missing, run tools/setup_race_windows.sh first.

command -v go >/dev/null 2>&1 || { echo "ERROR: go not found on PATH." >&2; exit 1; }

if ! command -v gcc >/dev/null 2>&1; then
  cat >&2 <<'EOF'
ERROR: gcc not found on PATH — the race detector needs a C compiler.
Set it up once with:
  tools/setup_race_windows.sh
EOF
  exit 1
fi

# Default to the whole module when no package/flags are supplied.
if [[ $# -eq 0 ]]; then
  set -- ./...
fi

echo "==> CGO_ENABLED=1 go test -race $*"
CGO_ENABLED=1 exec go test -race "$@"
