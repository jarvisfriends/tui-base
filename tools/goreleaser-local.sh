#!/usr/bin/env bash
set -euo pipefail

if ! command -v goreleaser >/dev/null 2>&1; then
  echo "goreleaser not found in PATH; install from https://goreleaser.com/install/"
  exit 1
fi

echo "Running goreleaser in snapshot mode (local build)"
goreleaser release --snapshot --clean
