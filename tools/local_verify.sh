#!/usr/bin/env bash
set -euo pipefail

echo "==> gofmt (check only)"
mapfile -t GO_FILES < <(git ls-files '*.go')
UNFORMATTED=$(gofmt -l "${GO_FILES[@]}" 2>/dev/null || true)
if [[ -n "${UNFORMATTED}" ]]; then
  echo "ERROR: gofmt required for:"
  echo "${UNFORMATTED}"
  exit 1
fi

echo "==> golangci-lint"
for target_os in windows linux; do
  echo "==> golangci-lint (GOOS=${target_os})"
  GOOS="${target_os}" GOARCH=amd64 golangci-lint run ./...
done

if command -v shellcheck >/dev/null 2>&1; then
  echo "==> shellcheck"
  mapfile -t SH_FILES < <(find tools .githooks -type f \( -name '*.sh' -o -name 'pre-commit' \) 2>/dev/null | sort)
  if [[ ${#SH_FILES[@]} -gt 0 ]]; then
    shellcheck "${SH_FILES[@]}"
  fi
else
  echo "WARN: shellcheck not found; skipping shell lint (CI still enforces this)."
fi

if command -v markdownlint-cli2 >/dev/null 2>&1; then
  echo "==> markdownlint-cli2"
  mapfile -t MD_FILES < <(git ls-files '*.md')
  if [[ ${#MD_FILES[@]} -gt 0 ]]; then
    markdownlint-cli2 "${MD_FILES[@]}"
  fi
elif command -v npx >/dev/null 2>&1; then
  echo "==> markdownlint-cli2 (via npx)"
  mapfile -t MD_FILES < <(git ls-files '*.md')
  if [[ ${#MD_FILES[@]} -gt 0 ]]; then
    npx --yes markdownlint-cli2 "${MD_FILES[@]}"
  fi
else
  echo "WARN: markdownlint-cli2/npx not found; skipping markdown lint (CI still enforces this)."
fi

if command -v actionlint >/dev/null 2>&1; then
  echo "==> actionlint"
  actionlint
else
  echo "WARN: actionlint not found; skipping workflow lint (CI still enforces this)."
fi

echo "==> go mod verify"
go mod verify

echo "==> go mod tidy (drift check)"
go mod tidy
if ! git diff --exit-code go.mod go.sum; then
  echo "ERROR: go mod tidy changed files — commit the result before pushing"
  exit 1
fi

echo "==> go fix (drift check)"
if ! go fix -diff ./...; then
  echo ""
  echo "ERROR: go fix found suggestions. Run: go fix ./..."
  echo "Then review the changes with 'git diff' and commit them."
  exit 1
fi

echo "==> go vet"
go vet ./...

echo "==> go test -race"
go test -race ./... -v

echo "Local verification passed."
