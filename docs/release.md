# Release

Releases are tag-driven and built by GoReleaser, and binary releases are on
hold. A `vX.Y.Z` tag records a version and publishes nothing; a binary release is
built from a tag only when dispatched by hand with a confirmation (see
[Cutting a release](#cutting-a-release)). Until binary releases resume, users
build from source ([installation](installation.md#build-from-source)). The CI
GoReleaser version is pinned (`~> v2.4`) so the config schema and tool stay in
sync.

## Local deliverables (no publishing)

Build the binaries and archives locally, useful for testing what users would
get:

```sh
make cross    # build/<os>/probaci_<os>_<arch>[.exe] plus build/checksums.txt
make dist     # the same, plus archives and their checksums in build/archives/
```

Both run `src/scripts/build-all.sh`, never GoReleaser, and publish nothing.
`build/` holds one folder per OS (`macos`, never `darwin`) and nothing else;
`make check-build` fails on anything else, and
`cd build && shasum -a 256 -c checksums.txt` verifies the binaries.

To exercise the GoReleaser config end to end, `make bundle` runs
`goreleaser release --config src/.goreleaser.yaml --snapshot --clean --skip=publish,sign,sbom`
into GoReleaser's scratch dist, `.cache/goreleaser/` (gitignored, never
`build/`), with no OIDC/cosign/registry/syft needed. Validate the config with
`make release-check` (`goreleaser check --config src/.goreleaser.yaml`).

## Install scripts

`src/scripts/install.sh` / `src/scripts/install.ps1` download the matching release archive,
verify its sha256 against `checksums.txt`, and install onto PATH. Their asset
names must match the GoReleaser archive `name_template`
(`probaci_<version>_<os>_<arch>`); keep them in sync. Test a script against a
local bundle without publishing:

```sh
make bundle
PROBACI_VERSION=0.0.0-SNAPSHOT-<commit> PROBACI_BASE_URL="file://$PWD/.cache/goreleaser" \
  PROBACI_INSTALL_DIR=/tmp/probaci sh src/scripts/install.sh
```

## Pinning tool-image digests

The built-in tool images ship pinned by digest (`src/internal/tool/digests.json`,
embedded into the binary). Refresh the pins periodically — run in a connected
environment with a container runtime + python3 and a built `probaci`:

```sh
make build
PROBACI="$(ls build/*/probaci)" ./src/scripts/pin-digests.sh > src/internal/tool/digests.json
```

Images that fail to pull are reported and left unpinned (advisory). Commit the
regenerated `digests.json`.

## Cutting a release

1. Update `CHANGELOG.md` (move Unreleased → the new version).
2. Tag and push:
   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```
   Pushing a tag publishes nothing: binary releases are on hold, and probaci is
   built from source until the owner decides otherwise.
3. Only when a binary release has been approved for this version, dispatch the
   `release` workflow on the tag:
   ```sh
   gh workflow run release.yml --repo simtabi/probaci --ref vX.Y.Z -f confirm=release-go
   ```
   `vX.Y.Z` must be a tag that is already on GitHub. This publishes for real, so
   run it only for a version the owner approved. The workflow's first step
   refuses to run without `confirm=release-go`, or on anything but a `v[0-9]*`
   tag, before anything is built.
4. The `release` workflow runs GoReleaser, which builds the matrix, signs with
   cosign (keyless via OIDC), attaches an SBOM and SLSA provenance, and publishes
   the GitHub Release, deb/rpm, and Homebrew/Scoop manifests. No container image
   is built.

## Build matrix

linux `amd64/arm64/arm(v6,v7)/386`, darwin `amd64/arm64`, windows `amd64/arm64/386`.
Pure Go, `CGO_ENABLED=0`.

## First release (short form)

1. Make the repo public on GitHub.
2. Create the `pypi`/`npm`-equivalent here: a GitHub Release flow via OIDC (no
   long-lived registry credentials).
3. Add the write deploy keys `HOMEBREW_TAP_DEPLOY_KEY` (on `simtabi/homebrew-tap`) and `SCOOP_BUCKET_DEPLOY_KEY` (on `simtabi/scoop-bucket`), each a repository secret that reaches its one repository and nothing else (`TAP_GITHUB_TOKEN` is only a fallback, and the
   Winget pull request's token), plus any registry tokens as needed.
4. Tag `v0.1.0`, then dispatch the release on it as in
   [Cutting a release](#cutting-a-release).

The full org-wide gate is `/opensource/shipping-checklist.md` (local; not in any
repo). Run it before every release.

## Version stamping

The binary embeds version metadata via `-ldflags` into
`github.com/simtabi/probaci/src/internal/version`; `probaci version` prints it.

## Module path and tags

The Go module lives in `src/`, so its path is `github.com/simtabi/probaci/src`.
Go resolves a subdirectory module only through a tag named after the directory,
so every release carries two tags on the same commit: `vX.Y.Z`, which the release
runs from, and `src/vX.Y.Z`. The release workflow creates the `src/` tag itself
before GoReleaser runs; neither tag is ever moved.

---

[← Docs index](../README.md#documentation)
