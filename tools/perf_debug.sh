#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "== Inspector perf report =="
go test ./pages/debug -run TestInspectorPerfReport -count=1 -v

echo
echo "== Inspector benchmarks (idle + mouse motion) =="
go test ./pages/debug -run '^$' -bench 'BenchmarkInspector(Idle|MouseMotion)$' -benchmem -count=3

echo
echo "Tip: set optional guardrails with env vars before running:"
echo "  TUIBASE_MAX_IDLE_ALLOCS_PER_SEC"
echo "  TUIBASE_MAX_IDLE_GC_PER_SEC"
echo "  TUIBASE_MAX_MOUSE_ALLOCS_PER_SEC"
echo "  TUIBASE_MAX_MOUSE_GC_PER_SEC"
