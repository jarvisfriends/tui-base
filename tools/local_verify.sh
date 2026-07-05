#!/usr/bin/env bash
set -euo pipefail

# ─── helper: detect Go version mismatch (preflight) ──────────────────────────
# Some Go tools (govulncheck, golangci-lint, gorelease, …) load packages using
# the Go version they were compiled with. If that version differs from the Go
# on PATH you may see errors like:
#
#   "This application uses version go1.25 of the source-processing packages
#    but runs version go1.26 of 'go list'."
#
#   "Loading packages failed, possibly due to a mismatch between the Go
#    version used to build <tool> and the Go version on PATH."
#
# check_go_tool_version detects this before running the tool so the developer
# sees a clear rebuild reminder at the top of the output, not buried in errors.
check_go_tool_version() {
  local cmd="$1"
  local install_cmd="$2"
  local binary_path
  binary_path=$(command -v "$cmd" 2>/dev/null) || return 0  # not installed — handled elsewhere

  local tool_go_ver current_go_ver tool_mm current_mm
  # "go version -m <binary>" first line: "/path/to/binary: go1.25.4"
  tool_go_ver=$(go version -m "$binary_path" 2>/dev/null \
    | head -1 \
    | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' \
    | head -1 || true)
  current_go_ver=$(go version 2>/dev/null \
    | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' \
    | head -1 || true)

  # Compare major.minor only — a patch difference is fine; a minor bump is not.
  tool_mm=$(echo "$tool_go_ver"       | grep -oE 'go[0-9]+\.[0-9]+' || true)
  current_mm=$(echo "$current_go_ver" | grep -oE 'go[0-9]+\.[0-9]+' || true)

  if [[ -n "$tool_mm" && -n "$current_mm" && "$tool_mm" != "$current_mm" ]]; then
    echo "WARN: '$cmd' was built with $tool_go_ver but current Go is $current_go_ver."
    echo "      Version mismatch can cause package-load failures (see run below)."
    echo "      Rebuild it with the current Go toolchain:"
    echo "        $install_cmd"
    echo ""
  fi
}

# ─── helper: run a Go tool with inline mismatch detection ────────────────────
# Streams tool output live to the terminal. If the output contains a known
# version-mismatch error string, prints a rebuild hint directly below it so
# the developer always sees what to do without scrolling back to the preflight.
run_go_tool() {
  local cmd="$1"
  local install_cmd="$2"
  shift 2

  local out exit_code=0
  out=$(mktemp)

  # Stream output live; || prevents -e from aborting before we print the hint.
  # With pipefail, exit_code captures the exit status of $cmd (not tee).
  "$cmd" "$@" 2>&1 | tee "$out" || exit_code=$?

  if grep -qE \
    "This application uses version .* of the source-processing packages|mismatch between the Go version used to build" \
    "$out" 2>/dev/null; then
    echo ""
    echo "──────────────────────────────────────────────────────────────────────"
    echo "  Go version mismatch detected in the output above."
    echo "  '$cmd' was compiled with an older Go toolchain than what is on PATH."
    echo "  Rebuild it with your current Go version:"
    echo "    $install_cmd"
    echo "──────────────────────────────────────────────────────────────────────"
  fi

  rm -f "$out"
  return "$exit_code"
}

# ─── preflight: tool Go-version compatibility ─────────────────────────────────
# Run version checks upfront so mismatches appear at the top of the output,
# not buried after a cryptic failure mid-run.
echo "==> preflight: Go-version compatibility checks"
check_go_tool_version "golangci-lint" \
  "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
check_go_tool_version "gofumpt" \
  "go install mvdan.cc/gofumpt@latest"
check_go_tool_version "govulncheck" \
  "go install golang.org/x/vuln/cmd/govulncheck@latest"
check_go_tool_version "gorelease" \
  "go install golang.org/x/exp/cmd/gorelease@latest"
check_go_tool_version "actionlint" \
  "go install github.com/rhysd/actionlint/cmd/actionlint@latest"
check_go_tool_version "stringer" \
  "go install golang.org/x/tools/cmd/stringer@latest"

# ─── preflight: required generate tools ──────────────────────────────────────
# stringer is used by //go:generate stringer directives in this repo.
# Without it, the go generate drift check below is skipped.
if ! command -v stringer >/dev/null 2>&1; then
  echo "WARN: 'stringer' not found — go generate drift check will be skipped."
  echo "      Install with:"
  echo "        go install golang.org/x/tools/cmd/stringer@latest"
  echo "      Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
  echo ""
fi

echo "==> golangci-lint"

# Ensure golangci-lint is present and is v2. If users have an old v1 binary
# (installed via package manager or manual copy) it frequently errors; fail
# early with actionable instructions to remove v1 and install v2 via `go install`.
check_golangci_lint() {
  if ! command -v golangci-lint >/dev/null 2>&1; then
    echo "ERROR: 'golangci-lint' not found. Install v2 with:"
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
# Lint both GOOS targets like CI does, so platform-specific files
# (disk_windows.go vs disk_unix.go, terminal_win.go, …) surface issues before
# push regardless of the developer's OS (CI-9). Subshells keep the GOOS/GOARCH
# exports from leaking into the rest of the script.
echo "==> golangci-lint (GOOS=windows)"
# shellcheck disable=SC2030,SC2031  # subshell-local GOOS/GOARCH is the point
(
  export GOOS=windows GOARCH=amd64
  run_go_tool "golangci-lint" \
    "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" \
    run ./...
)
echo "==> golangci-lint (GOOS=linux)"
# shellcheck disable=SC2030,SC2031  # subshell-local GOOS/GOARCH is the point
(
  export GOOS=linux GOARCH=amd64
  run_go_tool "golangci-lint" \
    "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" \
    run ./...
)

if command -v shellcheck >/dev/null 2>&1; then
  echo "==> shellcheck"
  mapfile -t SH_FILES < <(find tools .githooks -type f \( -name '*.sh' -o -name 'pre-commit' \) 2>/dev/null | sort)
  if [[ ${#SH_FILES[@]} -gt 0 ]]; then
    shellcheck "${SH_FILES[@]}"
  fi
else
  echo "WARN: 'shellcheck' not found; skipping shell lint (CI still enforces this)."
  echo "      Install with one of the following (pick for your OS):"
  echo "        macOS:   brew install shellcheck"
  echo "        Windows: choco install shellcheck  OR  scoop install shellcheck"
  echo "        Debian:  sudo apt install shellcheck"
  echo "        Prebuilt binaries: https://github.com/koalaman/shellcheck/releases"
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
  echo "WARN: 'markdownlint-cli2' and 'npx' not found; skipping markdown lint (CI still enforces this)."
  echo "      Install with npm/yarn:"
  echo "        npm install -g markdownlint-cli2"
  echo "      Or rely on npx (bundled with npm); to get npx install Node.js/npm:"
  echo "        macOS:   brew install node"
  echo "        Windows: choco install nodejs  OR  scoop install nodejs"
  echo "        Debian:  sudo apt install nodejs npm"
fi

if command -v actionlint >/dev/null 2>&1; then
  echo "==> actionlint"
  actionlint
else
  echo "WARN: 'actionlint' not found; skipping workflow lint (CI still enforces this)."
  echo "      Install with:"
  echo "        go install github.com/rhysd/actionlint/cmd/actionlint@latest"
  echo "      Or use package managers:"
  echo "        macOS:   brew install actionlint"
  echo "        Windows: choco install actionlint  OR  scoop install actionlint"
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

# ─── go generate drift check ────────────────────────────────────────────────
# Skipped when stringer is not installed; the preflight section above already
# warned the developer. CI enforces this check unconditionally.
echo "==> go generate (drift check)"
if ! command -v stringer >/dev/null 2>&1; then
  echo "WARN: 'stringer' not found; skipping go generate drift check."
  echo "      Install with: go install golang.org/x/tools/cmd/stringer@latest"
else
  go generate ./...
  if ! git diff --exit-code; then
    echo ""
    echo "ERROR: go generate produced changes — run 'go generate ./...' and commit the result."
    exit 1
  fi
fi

echo "==> go vet"
go vet ./...

echo "==> go test (with race detector)"
CGO_ENABLED=1 go test -race ./... -v

# ─── govulncheck (security scan) ─────────────────────────────────────────────
if command -v govulncheck >/dev/null 2>&1; then
  echo "==> govulncheck"
  run_go_tool "govulncheck" \
    "go install golang.org/x/vuln/cmd/govulncheck@latest" \
    ./...
else
  echo "WARN: 'govulncheck' not found; skipping vulnerability scan (CI still enforces this)."
  echo "      Install with:"
  echo "        go install golang.org/x/vuln/cmd/govulncheck@latest"
  echo "      Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
fi

# ─── gorelease (API backward compatibility) ───────────────────────────────────
if command -v gorelease >/dev/null 2>&1; then
  echo "==> gorelease (API compatibility check)"
  # gorelease compares the current public API against the latest tagged
  # release. Before the first tag, it reports "no base version" — that's OK.
  run_go_tool "gorelease" \
    "go install golang.org/x/exp/cmd/gorelease@latest" \
    2>&1 || true
else
  echo "WARN: 'gorelease' not found; skipping API compatibility check."
  echo "      Install with:"
  echo "        go install golang.org/x/exp/cmd/gorelease@latest"
  echo "      Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
fi

echo "Local verification passed."
