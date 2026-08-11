# Dependency policy

## Selection

A new direct dependency needs to clear this bar before it lands:

- solves a real problem a stdlib or existing-dependency solution cannot,
- OSI-approved license compatible with MIT (checked by `dependency-review` in CI),
- actively maintained and pinned to a tagged release (no `main`-branch pins),
- no reduction in the project's platform support.

PRs adding a dependency must say why in the description. Prefer extending existing code over adding
dependencies; this project deliberately stays dependency-light.

## Tracking

- Direct and transitive dependencies are recorded in `go.mod`/`go.sum` (hash-pinned by the Go toolchain).
- An SPDX SBOM is published for every release archive.
- GitHub Actions are third-party dependencies too: every action is pinned by full commit SHA.

## Monitoring and updates

- **Dependabot** files weekly update PRs for Go modules and GitHub Actions.
- **govulncheck** runs in CI on every push/PR and fails on known vulnerabilities that are actually reachable.
- **dependency-review** blocks vulnerable or license-incompatible dependency changes on PRs.
- Remediation deadlines follow the thresholds in [SECURITY.md](../SECURITY.md).

## Removal

Dependencies that become unmaintained, redundant, or security liabilities are replaced or vendored out;
`go mod tidy` drift is enforced in CI so nothing unused lingers.
