#!/usr/bin/env bash
set -euo pipefail

# setup_race_linux.sh — prepare a Linux machine to run Go's race detector.
#
# Like the Windows variant, the race detector (`go test -race`) needs:
#   1. CGO_ENABLED=1
#   2. A working C compiler (gcc/cc) AND the libc development headers, so cgo
#      can link the race runtime. (race supports amd64/arm64/ppc64le/s390x.)
#
# Supports Ubuntu/Debian (apt), Fedora/RHEL (dnf/yum), Alpine (apk), Arch
# (pacman), and openSUSE (zypper). It verifies the toolchain, installs it if
# missing, then runs a small `go test -race` smoke build to prove it links.
#
# Usage:
#   tools/setup_race_linux.sh [--check] [--yes] [--method auto|apt|dnf|yum|apk|pacman|zypper]
#
#   --check        Only report status; never install.
#   --yes, -y      Install without the interactive confirmation prompt.
#   --method M     Force a package manager (default: auto-detect).
#
# Alpine note: on musl-based systems the race detector works on recent Go, but
# requires `gcc` + `musl-dev`. If linking fails, also ensure `libc-dev` is
# present (provided by the `build-base` meta-package).

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
      sed -n '3,25p' "$0"
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

info()  { printf '==> %s\n' "$*"; }
warn()  { printf 'WARN: %s\n' "$*" >&2; }
fail()  { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

# ── Pre-flight: Go present and a race-capable target ─────────────────────────
command -v go >/dev/null 2>&1 || fail "go not found on PATH. Install Go first: https://go.dev/dl/"

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
info "Go $(go env GOVERSION) — GOOS=${GOOS} GOARCH=${GOARCH}"

if [[ "${GOOS}" != "linux" ]]; then
  warn "GOOS is '${GOOS}', not 'linux'. This script targets Linux; run the matching"
  warn "setup script for your platform (e.g. tools/setup_race_windows.sh)."
fi

case "${GOARCH}" in
  amd64|arm64|ppc64le|s390x) ;;
  *) fail "the linux race detector requires GOARCH amd64, arm64, ppc64le, or s390x (got ${GOARCH})." ;;
esac

# Identify the distro for friendlier messaging (package selection is by manager).
DISTRO_PRETTY="unknown Linux"
if [[ -r /etc/os-release ]]; then
  # shellcheck source=/dev/null
  . /etc/os-release
  DISTRO_PRETTY="${PRETTY_NAME:-${NAME:-unknown Linux}}"
fi
info "Distro: ${DISTRO_PRETTY}"

# Privilege escalation: none if root, else sudo when available.
SUDO=""
if [[ "$(id -u)" -ne 0 ]]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  fi
fi

# ── C-toolchain detection ────────────────────────────────────────────────────
# A usable setup means a C compiler is on PATH. The libc headers are verified
# by the smoke build (which actually links the race runtime).
find_cc() {
  local cc
  for cc in cc gcc clang; do
    if command -v "${cc}" >/dev/null 2>&1; then
      command -v "${cc}"
      return 0
    fi
  done
  return 1
}

# ── Package-manager selection & install ──────────────────────────────────────
choose_method() {
  if [[ "${METHOD}" != "auto" ]]; then
    printf '%s\n' "${METHOD}"
    return 0
  fi
  local mgr
  for mgr in apt-get dnf yum apk pacman zypper; do
    if command -v "${mgr}" >/dev/null 2>&1; then
      # Normalise apt-get -> apt for display/case matching.
      [[ "${mgr}" == "apt-get" ]] && mgr="apt"
      printf '%s\n' "${mgr}"
      return 0
    fi
  done
  printf 'none\n'
}

print_manual_instructions() {
  cat >&2 <<'EOF'
No supported package manager was detected. Install a C compiler and the libc
development headers with your distro's tools, then re-run:

  Debian/Ubuntu : sudo apt-get install -y gcc libc6-dev
  Fedora/RHEL   : sudo dnf install -y gcc glibc-devel
  Alpine        : sudo apk add --no-cache gcc musl-dev
  Arch          : sudo pacman -S --needed gcc
  openSUSE      : sudo zypper install -y gcc glibc-devel
EOF
}

do_install() {
  local method="$1"
  info "Installing C toolchain via: ${method}${SUDO:+ (using sudo)}"
  case "${method}" in
    apt)
      ${SUDO} apt-get update
      ${SUDO} apt-get install -y gcc libc6-dev
      ;;
    dnf)
      ${SUDO} dnf install -y gcc glibc-devel
      ;;
    yum)
      ${SUDO} yum install -y gcc glibc-devel
      ;;
    apk)
      # musl-dev provides the musl libc headers needed to link the race runtime.
      ${SUDO} apk add --no-cache gcc musl-dev
      ;;
    pacman)
      ${SUDO} pacman -S --needed --noconfirm gcc
      ;;
    zypper)
      ${SUDO} zypper install -y gcc glibc-devel
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
    fail "C compiler missing and input is non-interactive. Re-run with --yes to install via '${method}', or --check to only report."
  fi
  printf 'Install the C toolchain via %s now? [y/N] ' "${method}"
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
if CC_PATH="$(find_cc)"; then
  info "Found C compiler: ${CC_PATH} ($("${CC_PATH}" -dumpmachine 2>/dev/null || echo 'unknown target'))"
else
  if [[ "${CHECK_ONLY}" -eq 1 ]]; then
    fail "no C compiler found (run without --check to install)."
  fi
  METHOD_RESOLVED="$(choose_method)"
  if [[ "${METHOD_RESOLVED}" == "none" ]]; then
    print_manual_instructions
    exit 1
  fi
  if [[ -z "${SUDO}" && "$(id -u)" -ne 0 ]]; then
    warn "Not running as root and 'sudo' was not found; the install step may fail."
  fi
  if confirm_install "${METHOD_RESOLVED}"; then
    do_install "${METHOD_RESOLVED}"
  else
    fail "declined install — cannot run the race detector without a C toolchain."
  fi
  hash -r 2>/dev/null || true
  if CC_PATH="$(find_cc)"; then
    info "C compiler is now available: ${CC_PATH}"
  else
    fail "install completed but no C compiler is on PATH — check the package manager output above."
  fi
fi

if [[ "${CHECK_ONLY}" -eq 1 ]]; then
  info "Check passed: C compiler present. (Skipping smoke test in --check mode.)"
  exit 0
fi

run_smoke_test

cat <<'EOF'

✔ Race detector is ready.

Run race tests with:
  tools/race_test.sh              # CGO_ENABLED=1 go test -race ./...
  tools/race_test.sh ./pages/...  # scope to specific packages
EOF
