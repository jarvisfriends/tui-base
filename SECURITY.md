# Security Policy

## Supported versions

Only the latest tagged release and the current `main` branch receive security fixes. Older releases reach
end of life the moment a newer release is published; they receive no further security updates. The support
policy for each release is therefore: supported until superseded.

## Reporting a vulnerability

Please use
[GitHub's private vulnerability reporting](https://github.com/jarvisfriends/tui-base/security)
for this repository rather than opening a public issue.

Our coordinated disclosure process:

- **Acknowledgement** within 7 days, **triage decision** within 14 days.
- We aim to release a fix within **90 days** of a confirmed report, and earlier for critical issues.
- We publish an advisory through
  [GitHub Security Advisories](https://github.com/jarvisfriends/tui-base/security) when the fix
  ships, including affected versions and workarounds.
- **Credit:** reporters are credited by name/handle in the advisory and release notes unless they ask not
  to be. We do not currently pay bounties.

## Vulnerability handling thresholds

These thresholds govern both dependency (SCA) and static-analysis (SAST) findings:

- **Critical/High** findings are fixed or mitigated before the next release, and in no case left unpatched
  on `main` for more than 14 days after confirmation.
- **Medium** findings are fixed within 60 days.
- **Low** findings are batched into regular maintenance.
- A finding that provably does not affect this project (e.g. vulnerable code path never invoked) may instead
  be documented as not-affected in the advisory/VEX notes of the next release, with justification.

Automated enforcement: `govulncheck`, CodeQL, `golangci-lint`, and secret scanning run in CI on every PR;
`dependency-review` blocks known-vulnerable or license-incompatible dependency changes at merge time; and
Dependabot files weekly update PRs. A release is not tagged while a known Critical/High finding is open.

## Secrets

No credentials, tokens, or private keys are stored in this repository. CI uses short-lived OIDC tokens for
release signing (keyless cosign); nothing long-lived exists to rotate. Secret scanning (gitleaks) runs in CI
and GitHub push protection is enabled for the repository.

## Release integrity

Release archives are checksummed, SBOMs are published per archive, and `checksums.txt` is signed with
keyless cosign. See [docs/release-verification.md](docs/release-verification.md) for verification steps.
