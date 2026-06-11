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

# Ensure golangci-lint is present and is v2. If users have an old v1 binary
# (installed via package manager or manual copy) it frequently errors; fail
# early with actionable instructions to remove v1 and install v2 via `go install`.
check_golangci_lint() {
  if ! command -v golangci-lint >/dev/null 2>&1; then
    echo "ERROR: golangci-lint not found. Install v2 with:"
    echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
    echo "Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
    exit 1
  fi
  ver=$(golangci-lint --version 2>&1 || true)
  # extract semantic version major component if present
  if [[ $ver =~ ([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    major=${BASH_REMATCH[1]}
  elif [[ $ver =~ v([0-9]+) ]]; then
    major=${BASH_REMATCH[1]}
  else
    major=""
  fi
  if [[ "$major" == "1" ]]; then
    echo "ERROR: Detected golangci-lint v1: $ver"
    echo "Remove the old v1 installation (examples):"
    echo "  brew uninstall golangci-lint" 
    echo "  choco uninstall golangci-lint" 
    echo "  scoop uninstall golangci-lint" 
    echo "  sudo apt remove golangci-lint" 
    echo "or delete the golangci-lint binary from your PATH."
    printf "\nInstall v2 (recommended) with:" 
    echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
    echo "Then ensure \$GOBIN or \$GOPATH/bin is on your PATH."
    exit 1
  fi
}

check_golangci_lint
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
  echo "To install shellcheck, consider one of the following (pick for your OS):"
  echo "  macOS:   brew install shellcheck"
  echo "  Windows: choco install shellcheck || scoop install shellcheck"
  echo "  Debian:  sudo apt install shellcheck"
  echo "  Or download prebuilt binaries: https://github.com/koalaman/shellcheck/releases"
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
  echo "WARN: markdownlint-cli2 and npx not found; skipping markdown lint (CI still enforces this)."
  echo "To install markdownlint-cli2 locally, you can use npm/yarn:" 
  echo "  npm install -g markdownlint-cli2"
  echo "Or rely on npx (bundled with npm) to run without global install; to get npx install Node.js/npm:" 
  echo "  macOS:   brew install node"
  echo "  Windows: choco install nodejs || scoop install nodejs"
  echo "  Debian:  sudo apt install nodejs npm"
fi

if command -v actionlint >/dev/null 2>&1; then
  echo "==> actionlint"
  actionlint
else
  echo "WARN: actionlint not found; skipping workflow lint (CI still enforces this)."
  echo "To install actionlint (Go):"
  echo "  go install github.com/rhysd/actionlint/cmd/actionlint@latest"
  echo "Or use package managers where available:"
  echo "  macOS:   brew install actionlint (if available)"
  echo "  Windows: choco install actionlint || scoop install actionlint"
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

echo "==> go test"
go test ./... -v

echo "Local verification passed."
