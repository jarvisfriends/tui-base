#!/usr/bin/env bash
# Runs once after the container is created (devcontainer postCreateCommand).
# Installs the extra Go tools that CI requires but are not in the base image.
set -euo pipefail

echo "→ installing Go tools..."
# renovate: datasource=go depName=golang.org/x/tools
go install golang.org/x/tools/cmd/stringer@v0.48.0
# renovate: datasource=go depName=golang.org/x/vuln
go install golang.org/x/vuln/cmd/govulncheck@v1.5.0
# renovate: datasource=go depName=github.com/golangci/golangci-lint/v2
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
# renovate: datasource=go depName=github.com/goreleaser/goreleaser/v2
go install github.com/goreleaser/goreleaser/v2@v2.16.0

echo "→ pre-fetching module dependencies..."
go mod download

which nslookup google.com || (echo "→ nslookup not found, installing..." && apt-get update && apt-get install -y dnsutils)
echo "→ done"
