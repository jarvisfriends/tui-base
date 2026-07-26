# Governance

This document describes how the **tui-base** project (an application framework for Bubble Tea v2 (routing, theming,
settings, logging, overlays)) is run.

## Roles

- **Maintainer** — has write/admin access to the repository, reviews and merges changes, triages security
  reports, and cuts releases. Maintainers are listed in [MAINTAINERS.md](MAINTAINERS.md).
- **Contributor** — anyone who submits issues, discussions, or pull requests. The contribution process is
  described in [CONTRIBUTING.md](CONTRIBUTING.md).

## Decision making

Day-to-day decisions happen in issues and pull requests. Anything that changes public API, security posture,
or release mechanics gets an issue or PR description explaining the trade-offs before it merges. If
consensus cannot be reached among maintainers, the lead maintainer decides.

## Code review

- Every change lands through a pull request; direct pushes to `main` are blocked by branch protection.
- All automated status checks (CI, lint, DCO, coverage gate) must pass before merge.
- Contributions from anyone who is not a maintainer are always reviewed by a maintainer before merge.
- While the project has a single maintainer, maintainer-authored PRs may merge on green CI alone; as soon as
  a second maintainer joins, non-author review becomes mandatory for every change before release.

## Access policy

- New collaborators start with the lowest access level that lets them do the work (triage or write, never
  admin by default).
- Escalated permissions are granted only after a maintainer has reviewed the person's sustained
  contributions and the request is recorded in an issue.
- All accounts with write access or above must have two-factor authentication enabled.
- Access is removed when no longer needed or after 12 months of inactivity.

## Continuity

The repository lives in the `jarvisfriends` GitHub organization rather than a personal account, so ownership
can be transferred without repository migration. If the lead maintainer is unreachable for an extended
period, organization owners may appoint a new maintainer from established contributors.

## Changes to this document

Changes to governance follow the same PR + review process as code.
