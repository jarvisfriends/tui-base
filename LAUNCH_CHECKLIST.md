# OSS launch checklist — snap / inspector / tui-base

Everything below the "Done locally" line is committed and waiting for you to push.
The rest needs you (repo settings, secrets, or accounts I can't touch).

## Done locally (signed off, ready to push)

- PII removed: `amarcum@gmail.com` stripped from CODE_OF_CONDUCT, MAINTAINERS, SECURITY-INSIGHTS in all repos; reporting now points at GitHub built-ins (Report content / Report abuse, private vulnerability reporting).
- DCO check now exempts `*[bot]@users.noreply.github.com` in all three repos (snap gets the DCO workflow for the first time). This is the only check that was blocking tui-base Dependabot PRs #55/#56.
- OSV-Scanner workflow added (pinned v2.3.8), renovate.json + `# renovate:` annotations so every `go install pkg@version` pin in scripts/workflows auto-updates; all remaining `@latest` pinned (gofumpt v0.11.0, actionlint v1.7.12, choco mingw 16.1.0, etc.).
- Auto-release: optional RELEASE_PAT checkout now guarded via job env (step-level `if:` on `secrets.*` is unreliable and actionlint-hostile).
- inspector: SECURITY.md now has links (Scorecard Security-Policy 4→10 lever); GOVERNANCE.md, MAINTAINERS.md, SECURITY-INSIGHTS.yml added for parity.
- gitleaks: clean on current files AND full history in all three repos.

## Push order

1. **snap**: local main has diverged (remote gained sonarqube.yml etc.). `git pull --rebase origin main`, resolve if needed, then push.
2. **inspector**: push main (7 commits ahead).
3. **tui-base**: push `openssf-quality` (updates PR #54); merge #54, then comment `@dependabot rebase` on #55/#56 — they'll go green (CI already passes; only DCO failed) — and merge.

## Fix auto-tagging (the actual root cause)

snap run #68 failed at checkout with `could not read Username for 'https://github.com'` — the **RELEASE_PAT secret exists but is invalid/expired** (an empty secret would have fallen back cleanly; a bad one hard-fails). In each repo: Settings → Secrets → Actions → replace `RELEASE_PAT` with a fresh fine-grained PAT (contents: read/write, scoped to these repos) — or delete it and the workflow now falls back to GITHUB_TOKEN (tag still created; release.yml then needs a manual run).

## Repo settings (5 min each, Settings → …)

- **Code security**: enable Private vulnerability reporting; confirm CodeQL default setup covers **Go + Actions** (snap PRs warn "2 configurations not found" — clears after the next main analysis once default setup is on); enable secret scanning + push protection.
- **Rules → Rulesets** (not classic branch protection — Scorecard can read rulesets anonymously): require PR before merging + required status checks (CI, DCO) on main. This moves Branch-Protection (3→~6) and starts feeding Code-Review.
- **General**: add description/topics (snap has no topics — discoverability), enable Discussions.
- Tag ruleset protecting `v*` tags.

## Accounts worth setting up (your marketplace question)

Worth it:
1. **Renovate (Mend) GitHub App** — required for the renovate.json I added to do anything. Free for OSS.
2. **bestpractices.dev** — you're at 28% / 18% / 19% (ids 13784/13785/13787). Finishing the "passing" questionnaire is the single biggest Scorecard lever (CII-Best-Practices 2→10); most criteria are already true of these repos, it's mostly form-filling.
3. **Keep Codacy** (already green: "0 new issues" on PRs).
4. **Fix Coveralls instead of adding Codecov** — the upload step exits 1 non-blocking in snap CI and tui-base's goveralls fails; check the repo token/config.

Skip (redundant with CodeQL + golangci-lint + OSV + govulncheck + gitleaks): Snyk, DeepSource, Semgrep Cloud, Qlty. Optional nice-to-have: StepSecurity Harden-Runner for egress auditing in CI.

## What you were forgetting

- **Your git author email is the PII**: history (and future commits) carry `amarcum@gmail.com`. If that matters, flip on Settings → Emails → "Keep my email addresses private" + "Block command line pushes that expose my email", and set `git config user.email <id>+amarcum@users.noreply.github.com`. Note DCO sign-offs must then use the noreply too. Current history keeps it (you chose no rewrite).
- snap has **no releases/tags yet** — Packaging and Signed-Releases score -1 until the first `v0.x` tag (release.yml + goreleaser/cosign are ready).
- tui-base releases before cosign was wired are unsigned; the next tag publishes `.sigstore.json` bundles. Consider `actions/attest-build-provenance` later for SLSA provenance.
- Scorecard checks you can't rush: Maintained (repo <90 days), Contributors (single-org), Code-Review (needs reviewed PRs going forward — the ruleset above starts the clock).
- Delete `snap/gitleaks.tgz` (binary tarball in the repo dir; now gitignored so it can't be committed).
- I pinned `@latest` in the untracked `tools/local_verify.sh` of brick-breaker, multi, network-vis, snake too (accidental but harmless — same fix they'll need; dash was reverted). Keep or revert as you like.
- README badges: OpenSSF Scorecard, Best Practices, Go Reference, Go Report Card (free, no account).
- Org hygiene: org profile README, 2FA requirement, and a `.github` org repo for default community health files so future repos inherit all of this for free.
