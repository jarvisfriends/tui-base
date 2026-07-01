#!/usr/bin/env bash
# Runs once after the container is created (devcontainer postCreateCommand).
# Installs the extra Go tools that CI requires but are not in the base image.
set -euo pipefail

echo "→ installing Go tools..."
go install golang.org/x/tools/cmd/stringer@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go install github.com/goreleaser/goreleaser/v2@latest

echo "→ pre-fetching module dependencies..."
go mod download

which nslookup google.com || (echo "→ nslookup not found, installing..." && apt-get update && apt-get install -y dnsutils)
echo "→ done"
