# Verifying releases

Every release of tui-base ships the `tui-base` binary and per-example demo binaries, a `checksums.txt` covering all
archives, an SPDX SBOM per
archive, and a Sigstore bundle `checksums.txt.sigstore.json` produced by keyless cosign signing in the
release workflow.

## 1. Verify the checksum signature (proves origin)

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/jarvisfriends/tui-base/.github/workflows/release.yml@refs/tags/<TAG>" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
```

`<TAG>` is the release tag, e.g. `v1.2.3`. A valid signature proves the file was produced by this
repository's release workflow for that tag — no maintainer-held keys exist to leak.

## 2. Verify your download against the checksums (proves integrity)

```bash
sha256sum --check --ignore-missing checksums.txt
```

Because the signature covers `checksums.txt` and `checksums.txt` covers every archive, a successful check
of both steps transitively verifies any archive.

## 3. Inspect the SBOM (optional)

Each archive has a matching `*.sbom.json` (SPDX). Feed it to your SCA tool of choice, e.g.:

```bash
grype sbom:./tui-base_<...>.sbom.json
```

## Release provenance

Releases are cut only from signed-off commits on `main` via a tag push; the release workflow requires CI to
pass first, builds with goreleaser using reproducible-build flags (`-trimpath`, commit timestamp), and signs
with the workflow's OIDC identity. Maintainer identities are listed in
[MAINTAINERS.md](../MAINTAINERS.md).
