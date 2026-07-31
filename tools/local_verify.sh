#!/usr/bin/env bash
set -euo pipefail

MODE="${VERIFY_MODE:-full}"
if [[ "${1:-}" == "--mode" ]]; then
  MODE="${2:-}"
  shift 2
fi
if [[ "$MODE" != "fast" && "$MODE" != "full" ]]; then
  echo "ERROR: invalid mode '$MODE' (expected: fast|full)"
  exit 1
fi

echo "==> local verify mode: $MODE"

export GOWORK=off

echo "==> gofumpt (check only)"
if ! command -v gofumpt >/dev/null 2>&1; then
  echo "ERROR: 'gofumpt' not found. Install with:"
  # renovate: datasource=go depName=mvdan.cc/gofumpt
  echo "  go install mvdan.cc/gofumpt@v0.11.0"
  echo "Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
  exit 1
fi
mapfile -t GO_FILES < <(git ls-files '*.go')
if [[ ${#GO_FILES[@]} -gt 0 ]]; then
  UNFORMATTED=$(gofumpt -l "${GO_FILES[@]}" 2>/dev/null || true)
  if [[ -n "${UNFORMATTED}" ]]; then
    echo "ERROR: gofumpt required for:"
    echo "${UNFORMATTED}"
    exit 1
  fi
fi

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
  # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
  "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
check_go_tool_version "gofumpt" \
  # renovate: datasource=go depName=mvdan.cc/gofumpt
  "go install mvdan.cc/gofumpt@v0.11.0"
check_go_tool_version "govulncheck" \
  # renovate: datasource=go depName=golang.org/x/vuln
  "go install golang.org/x/vuln/cmd/govulncheck@v1.5.0"
check_go_tool_version "gorelease" \
  # renovate: datasource=go depName=golang.org/x/exp
  "go install golang.org/x/exp/cmd/gorelease@v0.0.0-20260718201538-764159d718ef"
check_go_tool_version "actionlint" \
  # renovate: datasource=go depName=github.com/rhysd/actionlint
  "go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"
check_go_tool_version "stringer" \
  # renovate: datasource=go depName=golang.org/x/tools
  "go install golang.org/x/tools/cmd/stringer@v0.47.0"

# ─── preflight: required generate tools ──────────────────────────────────────
# stringer is used by //go:generate stringer directives in this repo.
# Without it, the go generate drift check below is skipped.
if ! command -v stringer >/dev/null 2>&1; then
  echo "WARN: 'stringer' not found — go generate drift check will be skipped."
  echo "      Install with:"
  # renovate: datasource=go depName=golang.org/x/tools
  echo "        go install golang.org/x/tools/cmd/stringer@v0.47.0"
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
    # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
    echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
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
    # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
    echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2"
    echo "Then ensure \$GOBIN or \$GOPATH/bin is on your PATH."
    exit 1
  fi
}

check_golangci_lint
if [[ "$MODE" == "full" ]]; then
  # Lint both GOOS targets like CI does, so platform-specific files
  # (disk_windows.go vs disk_unix.go, terminal_win.go, …) surface issues before
  # push regardless of the developer's OS (CI-9). Subshells keep the GOOS/GOARCH
  # exports from leaking into the rest of the script.
  echo "==> golangci-lint (GOOS=windows)"
  # shellcheck disable=SC2030,SC2031  # subshell-local GOOS/GOARCH is the point
  (
    export GOOS=windows GOARCH=amd64
    run_go_tool "golangci-lint" \
      # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
      "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" \
      run ./...
  )
  echo "==> golangci-lint (GOOS=linux)"
  # shellcheck disable=SC2030,SC2031  # subshell-local GOOS/GOARCH is the point
  (
    export GOOS=linux GOARCH=amd64
    run_go_tool "golangci-lint" \
      # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
      "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" \
      run ./...
  )
else
  # Fast mode lints the developer's native platform, so use the host's real
  # GOARCH rather than forcing amd64 — on arm64 machines a cross-arch lint can
  # analyze the wrong build constraints or fail on tag/cgo differences.
  current_goos=$(go env GOOS)
  current_goarch=$(go env GOARCH)
  echo "==> golangci-lint (GOOS=${current_goos} GOARCH=${current_goarch})"
  # shellcheck disable=SC2030,SC2031  # subshell-local GOOS/GOARCH is intentional
  (
    export GOOS="$current_goos" GOARCH="$current_goarch"
    run_go_tool "golangci-lint" \
      # renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
      "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" \
      run ./...
  )
fi

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
  # renovate: datasource=go depName=github.com/rhysd/actionlint
  echo "        go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"
  echo "      Or use package managers:"
  echo "        macOS:   brew install actionlint"
  echo "        Windows: choco install actionlint  OR  scoop install actionlint"
fi

echo "==> go mod verify"
go mod verify

if [[ "$MODE" == "full" ]]; then
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
fi

# ─── go generate drift check ────────────────────────────────────────────────
# Skipped when stringer is not installed; the preflight section above already
# warned the developer. CI enforces this check unconditionally.
echo "==> go generate (drift check)"
if [[ "$MODE" == "full" ]]; then
  if ! command -v stringer >/dev/null 2>&1; then
    echo "WARN: 'stringer' not found; skipping go generate drift check."
    # renovate: datasource=go depName=golang.org/x/tools
    echo "      Install with: go install golang.org/x/tools/cmd/stringer@v0.47.0"
  else
    go generate ./...
    if ! git diff --exit-code; then
      echo ""
      echo "ERROR: go generate produced changes — run 'go generate ./...' and commit the result."
      exit 1
    fi
  fi
else
  echo "==> go generate (fast mode): skipped"
fi

echo "==> go vet"
go vet ./...

if [[ "$MODE" == "full" ]]; then
  echo "==> go test (with race detector)"
  CGO_ENABLED=1 go test -race ./... -v
else
  echo "==> go test"
  go test ./... -v
fi

# ─── govulncheck (security scan) ─────────────────────────────────────────────
if command -v govulncheck >/dev/null 2>&1; then
  if [[ "$MODE" == "full" ]]; then
    echo "==> govulncheck"
    run_go_tool "govulncheck" \
      # renovate: datasource=go depName=golang.org/x/vuln
      "go install golang.org/x/vuln/cmd/govulncheck@v1.5.0" \
      ./...
  else
    echo "==> govulncheck (fast mode): skipped"
  fi
else
  echo "WARN: 'govulncheck' not found; skipping vulnerability scan (CI still enforces this)."
  echo "      Install with:"
  # renovate: datasource=go depName=golang.org/x/vuln
  echo "        go install golang.org/x/vuln/cmd/govulncheck@v1.5.0"
  echo "      Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
fi

# ─── nested tool modules (build + vet) ────────────────────────────────────────
# tools/* are standalone modules, so the root ./... sweeps above never touch
# them — a broken tool module sailed through local verify until CI's
# dependency review tripped on it.
echo "==> nested modules (build + vet)"
while IFS= read -r modfile; do
  moddir=$(dirname "$modfile")
  [[ "$moddir" == "." ]] && continue
  echo "--- $moddir"
  (cd "$moddir" && go build ./... && go vet ./...)
done < <(git ls-files | grep -E '(^|/)go\.mod$')

# ─── dependency review (CI parity: vulnerabilities + scorecards) ─────────────
# CI's actions/dependency-review-action scans every go.mod in the repo — the
# nested tools/* modules included — for (a) known vulnerabilities and (b)
# OpenSSF Scorecards below the repo threshold on changed dependencies. Neither
# had a local equivalent (govulncheck above walks only the main module's call
# graph), so oksvg/rasterx, resvg-go, and docker/docker all had to round-trip
# through PR pushes to be seen. Vulnerabilities fail the run like CI does;
# low scorecards warn. The scorecard sweep here checks every direct dep, a
# superset of CI's changed-deps-only view.
SCORECARD_THRESHOLD="${SCORECARD_THRESHOLD:-3.0}"

# resolve_scorecard_repo maps a Go module path to the github.com/{owner}/{repo}
# slug the scorecard API understands: native GitHub paths and golang.org/x/*
# directly, other vanity hosts through their go-import meta tag (best effort).
resolve_scorecard_repo() {
  local mod="$1"
  case "$mod" in
    github.com/*)
      echo "$mod" | cut -d/ -f1-3
      ;;
    golang.org/x/*)
      echo "github.com/golang/$(echo "${mod#golang.org/x/}" | cut -d/ -f1)"
      ;;
    *)
      # Best effort: failures just produce an empty slug -> "skip" below.
      curl -fsS --max-time 10 "https://${mod}?go-get=1" 2>/dev/null \
        | sed -n 's|.*go-import[^>]*https://\(github\.com/[^" ]*\).*|\1|p' \
        | head -1 | sed 's|\.git$||' | cut -d/ -f1-3 || true
      ;;
  esac
  return 0
}

if command -v govulncheck >/dev/null 2>&1; then
  echo "==> dependency review (all modules)"
  while IFS= read -r modfile; do
    moddir=$(dirname "$modfile")
    echo "--- $moddir: govulncheck -scan module"
    # Module-level scan flags vulnerable dependency versions regardless of
    # reachability — the same bar the CI dependency review applies.
    # govulncheck loads the current-directory package even for module scans,
    # so run from the module's first package dir (module roots without .go
    # files error out otherwise).
    pkgdir=$( (cd "$moddir" && go list -f '{{.Dir}}' ./... 2>/dev/null | head -1) || true)
    [[ -z "$pkgdir" ]] && pkgdir="$moddir"
    (cd "$pkgdir" && run_go_tool "govulncheck" \
      # renovate: datasource=go depName=golang.org/x/vuln
      "go install golang.org/x/vuln/cmd/govulncheck@v1.5.0" \
      -scan module)

    echo "--- $moddir: OpenSSF Scorecards (direct deps, threshold $SCORECARD_THRESHOLD)"
    while IFS= read -r dep; do
      repo=$(resolve_scorecard_repo "$dep")
      if [[ -z "$repo" ]]; then
        echo "    skip  $dep (no GitHub mapping for scorecard)"
        continue
      fi
      # The aggregate "score" field precedes the per-check scores in the
      # response, so the first match is the overall scorecard.
      score=$(curl -fsS --max-time 10 "https://api.securityscorecards.dev/projects/${repo}" \
        2>/dev/null | grep -o '"score":-\?[0-9.]*' | head -1 | cut -d: -f2 || true)
      if [[ -z "$score" ]]; then
        echo "    skip  $dep (no scorecard data for $repo)"
        continue
      fi
      if awk -v s="$score" -v t="$SCORECARD_THRESHOLD" 'BEGIN{exit !(s<t)}'; then
        echo "    WARN  $dep scores $score (< $SCORECARD_THRESHOLD) — CI dependency review flags this"
      else
        echo "    ok    $dep ($score)"
      fi
    done < <(cd "$moddir" && awk '
      /^require \(/ {inreq=1; next}
      inreq && /^\)/ {inreq=0; next}
      inreq && !/\/\/ indirect/ && NF >= 2 {print $1}
      /^require [^(]/ && !/\/\/ indirect/ {print $2}
    ' go.mod)
  done < <(git ls-files | grep -E '(^|/)go\.mod$')
else
  echo "WARN: 'govulncheck' not found; skipping dependency review (CI still enforces this)."
fi

# ─── gorelease (API backward compatibility) ───────────────────────────────────
if command -v gorelease >/dev/null 2>&1; then
  echo "==> gorelease (API compatibility check)"
  # gorelease compares the current public API against the latest tagged
  # release. Before the first tag, it reports "no base version" — that's OK.
  run_go_tool "gorelease" \
    # renovate: datasource=go depName=golang.org/x/exp
    "go install golang.org/x/exp/cmd/gorelease@v0.0.0-20260718201538-764159d718ef" \
    2>&1 || true
else
  echo "WARN: 'gorelease' not found; skipping API compatibility check."
  echo "      Install with:"
  # renovate: datasource=go depName=golang.org/x/exp
  echo "        go install golang.org/x/exp/cmd/gorelease@v0.0.0-20260718201538-764159d718ef"
  echo "      Ensure \$GOBIN or \$GOPATH/bin is on your PATH."
fi

echo "Local verification passed."
