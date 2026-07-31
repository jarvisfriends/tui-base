#!/usr/bin/env bash
set -euo pipefail

# setup_race_windows.sh — prepare a Windows machine to run Go's race detector.
#
# The race detector (`go test -race`) needs only two things on Windows:
#   1. CGO_ENABLED=1
#   2. A working 64-bit mingw-w64 `gcc` on PATH (race supports amd64/arm64 only).
#
# A single WinLibs install provides the whole C toolchain — there is no longer
# list of dependencies beyond a functioning 64-bit gcc.
#
# This script verifies the toolchain, installs it if missing (WinLibs via
# winget by default; choco/scoop/MSYS2 also supported), then runs a small
# `go test -race` smoke build to prove everything links.
#
# Usage:
#   tools/setup_race_windows.sh [--check] [--yes] [--method auto|winget|choco|scoop|msys2]
#
#   --check        Only report status; never install.
#   --yes, -y      Install without the interactive confirmation prompt.
#   --method M     Force an install method (default: auto-detect).

ASSUME_YES=0
CHECK_ONLY=0
METHOD="auto"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check) CHECK_ONLY=1 ;;
    -y|--yes) ASSUME_YES=1 ;;
    --method)
      shift
      METHOD="${1:-auto}"
      ;;
    --method=*) METHOD="${1#*=}" ;;
    -h|--help)
      sed -n '3,20p' "$0"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

# WinLibs POSIX/UCRT: self-contained mingw-w64 gcc, ideal for CGO + race.
WINGET_ID="BrechtSanders.WinLibs.POSIX.UCRT"

info()  { printf '==> %s\n' "$*"; }
warn()  { printf 'WARN: %s\n' "$*" >&2; }
fail()  { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# ── Pre-flight: Go present and a race-capable target ─────────────────────────
command -v go >/dev/null 2>&1 || fail "go not found on PATH. Install Go first: https://go.dev/dl/"

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
info "Go $(go env GOVERSION) — GOOS=${GOOS} GOARCH=${GOARCH}"

if [[ "${GOOS}" != "windows" ]]; then
  warn "GOOS is '${GOOS}', not 'windows'. This script targets Windows; on Linux/macOS the"
  warn "default system C compiler usually already satisfies the race detector."
fi

# Map GOARCH -> the substring gcc -dumpmachine should report.
case "${GOARCH}" in
  amd64) WANT_TRIPLE="x86_64" ;;
  arm64) WANT_TRIPLE="aarch64" ;;
  *) fail "the race detector requires GOARCH amd64 or arm64 (got ${GOARCH})." ;;
esac

# ── gcc detection ────────────────────────────────────────────────────────────
# Returns 0 and prints the gcc path if a usable 64-bit gcc is found, else 1.
find_usable_gcc() {
  local gcc_path machine
  if ! gcc_path="$(command -v gcc 2>/dev/null)"; then
    return 1
  fi
  machine="$(gcc -dumpmachine 2>/dev/null || true)"
  if [[ "${machine}" != *"${WANT_TRIPLE}"* ]]; then
    warn "gcc at '${gcc_path}' reports target '${machine}', which does not match GOARCH=${GOARCH} (${WANT_TRIPLE})."
    return 1
  fi
  printf '%s\n' "${gcc_path}"
  return 0
}

# After a winget/portable install, the new PATH entry is not visible in this
# shell. Search WinGet's link/package dirs (and common manual locations) and
# prepend the directory holding a 64-bit gcc to PATH for the rest of this run.
locate_and_add_gcc_to_path() {
  hash -r 2>/dev/null || true
  if find_usable_gcc >/dev/null 2>&1; then
    return 0
  fi

  local localappdata candidates=() found dir
  localappdata="$(cygpath -u "${LOCALAPPDATA:-}" 2>/dev/null || true)"

  if [[ -n "${localappdata}" ]]; then
    candidates+=("${localappdata}/Microsoft/WinGet/Links")
    candidates+=("${localappdata}/Microsoft/WinGet/Packages")
  fi
  candidates+=("/c/msys64/mingw64/bin" "/c/mingw64/bin" "/c/ProgramData/mingw64/bin")

  for dir in "${candidates[@]}"; do
    [[ -d "${dir}" ]] || continue
    found="$(find "${dir}" -maxdepth 6 -type f -iname 'gcc.exe' -print -quit 2>/dev/null || true)"
    if [[ -n "${found}" ]]; then
      PATH="$(dirname "${found}"):${PATH}"
      export PATH
      hash -r 2>/dev/null || true
      find_usable_gcc >/dev/null 2>&1 && return 0
    fi
  done
  return 1
}

# ── Install ──────────────────────────────────────────────────────────────────
choose_method() {
  if [[ "${METHOD}" != "auto" ]]; then
    printf '%s\n' "${METHOD}"
    return 0
  fi
  if command -v winget >/dev/null 2>&1; then printf 'winget\n'; return 0; fi
  if command -v choco  >/dev/null 2>&1; then printf 'choco\n';  return 0; fi
  if command -v scoop  >/dev/null 2>&1; then printf 'scoop\n';  return 0; fi
  if command -v pacman >/dev/null 2>&1; then printf 'msys2\n';  return 0; fi
  printf 'none\n'
}

print_manual_instructions() {
  cat >&2 <<'EOF'
No supported package manager was found to install gcc automatically.
Install a 64-bit mingw-w64 toolchain by one of these means, then re-run:

  winget : winget install --id BrechtSanders.WinLibs.POSIX.UCRT -e
  choco  : choco install mingw --version=16.1.0 -y
  scoop  : scoop install gcc
  MSYS2  : winget install --id MSYS2.MSYS2 -e
           then in the MSYS2 shell: pacman -S --needed mingw-w64-x86_64-gcc
           and add C:\msys64\mingw64\bin to PATH

Or download a standalone build from https://winlibs.com/ and add its bin/ to PATH.
EOF
}

do_install() {
  local method="$1"
  info "Installing mingw-w64 gcc via: ${method}"
  case "${method}" in
    winget)
      winget install --id "${WINGET_ID}" -e --accept-source-agreements --accept-package-agreements
      ;;
    choco)
      choco install mingw --version=16.1.0 -y
      ;;
    scoop)
      scoop install gcc
      ;;
    msys2)
      command -v pacman >/dev/null 2>&1 || fail "MSYS2 'pacman' not found in this shell. Run this from the MSYS2 shell, or install MSYS2 first: winget install --id MSYS2.MSYS2 -e"
      pacman -S --needed --noconfirm mingw-w64-x86_64-gcc
      ;;
    none|*)
      print_manual_instructions
      exit 1
      ;;
  esac
}

confirm_install() {
  local method="$1"
  [[ "${ASSUME_YES}" -eq 1 ]] && return 0
  if [[ ! -t 0 ]]; then
    fail "gcc is missing and input is non-interactive. Re-run with --yes to install via '${method}', or --check to only report."
  fi
  printf 'Install a 64-bit mingw-w64 gcc toolchain via %s now? [y/N] ' "${method}"
  local reply
  read -r reply
  [[ "${reply}" =~ ^[Yy] ]]
}

# ── Smoke test: build & run a trivial test under -race ───────────────────────
run_smoke_test() {
  local tmp
  tmp="$(mktemp -d)"
  # shellcheck disable=SC2064  # expand tmp now so the trap removes the right dir
  trap "rm -rf '${tmp}'" RETURN

  cat > "${tmp}/go.mod" <<EOF
module racecheck

go 1.26
EOF

  cat > "${tmp}/race_test.go" <<'EOF'
package racecheck

import (
	"sync"
	"testing"
)

// Exercises the race runtime with correctly synchronized goroutines so the
// build proves the toolchain links without reporting a (spurious) race.
func TestRaceToolchain(t *testing.T) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	n := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			n++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if n != 8 {
		t.Fatalf("got %d, want 8", n)
	}
}
EOF

  info "Running smoke test: CGO_ENABLED=1 go test -race"
  ( cd "${tmp}" && CGO_ENABLED=1 go test -race . )
}

# ── Main ──────────────────────────────────────────────────────────────────────
if GCC_PATH="$(find_usable_gcc)"; then
  info "Found usable gcc: ${GCC_PATH} ($(gcc -dumpmachine))"
else
  if [[ "${CHECK_ONLY}" -eq 1 ]]; then
    fail "no usable 64-bit gcc found (run without --check to install)."
  fi
  METHOD_RESOLVED="$(choose_method)"
  if [[ "${METHOD_RESOLVED}" == "none" ]]; then
    print_manual_instructions
    exit 1
  fi
  if confirm_install "${METHOD_RESOLVED}"; then
    do_install "${METHOD_RESOLVED}"
  else
    fail "declined install — cannot run the race detector without a C toolchain."
  fi
  if locate_and_add_gcc_to_path; then
    info "gcc is now available: $(command -v gcc) ($(gcc -dumpmachine))"
  else
    warn "Installed gcc but could not locate it from this shell."
    warn "Open a NEW terminal so the updated PATH is picked up, then re-run with --check."
    exit 1
  fi
fi

if [[ "${CHECK_ONLY}" -eq 1 ]]; then
  info "Check passed: toolchain present. (Skipping smoke test in --check mode.)"
  exit 0
fi

run_smoke_test

cat <<'EOF'

✔ Race detector is ready.

Run race tests with:
  tools/race_test.sh              # CGO_ENABLED=1 go test -race ./...
  tools/race_test.sh ./pages/...  # scope to specific packages

If you opened this shell before installing gcc, start a NEW terminal so the
updated PATH is inherited for future runs.
EOF
