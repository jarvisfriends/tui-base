#!/usr/bin/env bash
set -euo pipefail

# ─── cSpell: spell-check the whole repo ──────────────────────────────────────
# Requires cspell. The easiest way to get it is via npx (bundled with npm);
# alternatively install globally with: npm install -g cspell
#
# Run this script to check all tracked files for unknown words before opening
# a PR, or any time you want a clean Problems tab without having to open every
# file individually in VS Code.
#
# Configuration lives in cspell.json at the repo root. Add new domain-specific
# words there rather than using inline disable comments.

if command -v cspell >/dev/null 2>&1; then
  CSPELL_CMD="cspell"
elif command -v npx >/dev/null 2>&1; then
  CSPELL_CMD="npx --yes cspell"
else
  echo "ERROR: 'cspell' not found and 'npx' is unavailable."
  echo "Install cspell with one of the following:"
  echo "  npm install -g cspell           # global install via npm"
  echo "  npx cspell ...                  # zero-install via npx (requires npm/Node)"
  echo "Node.js/npm can be installed with:"
  echo "  macOS:   brew install node"
  echo "  Windows: choco install nodejs  OR  scoop install nodejs  OR  winget install OpenJS.NodeJS"
  echo "  Debian:  sudo apt install nodejs npm"
  exit 1
fi

echo "==> cspell (whole-repo spell check)"
# --no-progress suppresses the per-file scan lines so only errors are shown.
# useGitignore in cspell.json automatically skips gitignored files (go.sum,
# coverage outputs, dist/, etc.) so we can safely pass "**" here.
$CSPELL_CMD "**" --no-progress
echo "Spell check passed."
