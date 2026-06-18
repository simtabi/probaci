# Release

Releases are tag-driven and built by GoReleaser. The CI GoReleaser version is
pinned (`~> v2.4`) so the config schema and tool stay in sync.

## Local deliverables (no publishing)

Build the downloadable archives locally — useful for testing the binaries users
will get:

```sh
task bundle        # or: make bundle
```

This runs `goreleaser release --snapshot --clean --skip=publish,sign,docker,sbom`
and writes `dist/probaci_<version>_<os>_<arch>.{tar.gz,zip}` plus `checksums.txt`
(no OIDC/cosign/registry/syft needed). Validate the config with
`task release-check` (`goreleaser check`).

## Install scripts

`install.sh` / `install.ps1` (repo root) download the matching release archive,
verify its sha256 against `checksums.txt`, and install onto PATH. Their asset
names must match the GoReleaser archive `name_template`
(`probaci_<version>_<os>_<arch>`); keep them in sync. Test a script against a
local bundle without publishing:

```sh
task bundle
PROBACI_VERSION=0.0.0-SNAPSHOT-<commit> PROBACI_BASE_URL="file://$PWD/dist" \
  PROBACI_INSTALL_DIR=/tmp/probaci sh install.sh
```

## Pinning tool-image digests

Before a release, pin the registry images by digest for tamper-evident,
reproducible runs (the trust gate flags unpinned images as advisory):

```sh
./scripts/pin-digests.sh > tools.pinned.json   # needs docker + python3, connected
```

Merge the resulting `tools` object into the registry defaults / shipped config.

## Cutting a release

1. Update `CHANGELOG.md` (move Unreleased → the new version).
2. Tag and push:
   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```
3. The `release` workflow runs GoReleaser, which builds the matrix, signs with
   cosign (keyless via OIDC), attaches an SBOM and SLSA provenance, and publishes
   the GitHub Release, container image, deb/rpm, and Homebrew/Scoop manifests.

## Build matrix

linux `amd64/arm64/arm(v6,v7)/386`, darwin `amd64/arm64`, windows `amd64/arm64/386`.
Pure Go, `CGO_ENABLED=0`.

## First release (short form)

1. Make the repo public on GitHub.
2. Create the `pypi`/`npm`-equivalent here: a GitHub Release flow via OIDC (no
   long-lived registry credentials).
3. Add `TAP_GITHUB_TOKEN` (Homebrew tap) and any registry tokens as needed.
4. Cut `v0.1.0`.

The full org-wide gate is `/opensource/shipping-checklist.md` (local; not in any
repo). Run it before every release.

## Version stamping

The binary embeds version metadata via `-ldflags` into
`internal/version`; `probaci version` prints it.

---

[← Docs index](../README.md#documentation)
