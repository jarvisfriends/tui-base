#!/usr/bin/env bash
set -euo pipefail

echo "==> gofmt (check only)"
UNFORMATTED=$(gofmt -l $(git ls-files '*.go') 2>/dev/null || true)
if [[ -n "${UNFORMATTED}" ]]; then
  echo "ERROR: gofmt required for:"
  echo "${UNFORMATTED}"
  exit 1
fi

echo "==> golangci-lint"
golangci-lint run ./...

echo "==> go vet"
go vet ./...

echo "==> go test -race"
go test -race ./... -v

echo "Local verification passed."
