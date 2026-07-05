#!/usr/bin/env bash
set -euo pipefail

LOG="dist/tinygo_build.log"
mkdir -p dist
echo "TinyGo build log" >"$LOG"

if ! command -v tinygo >/dev/null 2>&1; then
  echo "tinygo not found; skipping TinyGo builds" | tee -a "$LOG"
  exit 0
fi

echo "tinygo version: $(tinygo version)" >>"$LOG" 2>&1

echo "Attempting TinyGo wasm build..." | tee -a "$LOG"
if tinygo build -o dist/tui-base.wasm -target wasm ./cmd/tui-base 2>>"$LOG"; then
  echo "WASM build succeeded" >>"$LOG"
else
  echo "WASM build failed" >>"$LOG"
fi

echo "Attempting TinyGo native (linux/amd64) build..." | tee -a "$LOG"
if tinygo build -o dist/tui-base-tinygo-native -target linux/amd64 ./cmd/tui-base 2>>"$LOG"; then
  echo "native linux/amd64 build succeeded" >>"$LOG"
else
  echo "native linux/amd64 build failed" >>"$LOG"
fi

echo "TinyGo attempts logged to $LOG"

exit 0
