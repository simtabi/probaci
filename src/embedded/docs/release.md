# Release

Releases are tag-driven and built by GoReleaser. Binaries ship on the GitHub
release only: no Homebrew, Scoop, Winget, `go install`, image or any other
channel. A release builds in one of two ways:

```sh
git push origin vX.Y.Z                                              # builds and publishes vX.Y.Z
gh workflow run release.yml --repo simtabi/probaci -f tag=vX.Y.Z    # the same, for a tag already pushed
```

Because a pushed `vX.Y.Z` tag publishes, push one only for final code. A tag is
final: it is never moved, deleted or reused. Test builds use `vX.Y.Z-rc.N`, which
publishes a GitHub pre-release. The release runs simtabi/.github's shared
`go-release.yml`, which uses GoReleaser `~> v2`; validate the config locally with
`make release-check` before tagging.

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

1. Update `CHANGELOG.md` (move Unreleased → the new version) through a pull
   request. A final version with no `## [X.Y.Z]` section is refused.
2. Tag the merge commit on `main` and push the tag. The push builds and
   publishes the release:
   ```sh
   git -C ~/src/probaci tag vX.Y.Z
   git -C ~/src/probaci push origin vX.Y.Z
   ```
3. To build a release for a tag already on GitHub (`v0.1.0` was tagged before
   this workflow existed and has no release yet, or a re-run after a failed
   one), request it instead:
   ```sh
   gh workflow run release.yml --repo simtabi/probaci -f tag=v0.1.0
   ```
   The workflow refuses anything that is not a `vX.Y.Z` (or `vX.Y.Z-pre`) tag
   already on GitHub, before anything is built.
4. `release.yml` is a thin caller of simtabi/.github's shared `go-release.yml`,
   pinned to a commit. It refuses a GoReleaser config holding any other
   publishing block, mirrors `src/vX.Y.Z`, and runs GoReleaser, which builds the
   matrix, signs `checksums.txt` with cosign (keyless via OIDC), attaches an SBOM
   per archive, and publishes the GitHub Release with the binaries, archives and
   deb/rpm/apk packages. It then attests build provenance for every asset in
   `checksums.txt`. No container image is built.

## Build matrix

linux `amd64/arm64/arm(v6,v7)/386`, darwin `amd64/arm64`, windows `amd64/arm64/386`.
Pure Go, `CGO_ENABLED=0`.

## First release (short form)

1. Make the repo public on GitHub.
2. Create the `pypi`/`npm`-equivalent here: a GitHub Release flow via OIDC (no
   long-lived registry credentials).
3. No publishing secrets are needed: the release uses the run's own
   `GITHUB_TOKEN`. The tap, bucket and Winget credentials
   (`HOMEBREW_TAP_DEPLOY_KEY`, `SCOOP_BUCKET_DEPLOY_KEY`, `TAP_GITHUB_TOKEN`) are
   no longer read.
4. Request the release of the existing `v0.1.0` tag as in
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
before GoReleaser runs (it matches no release trigger, so it starts no second
run); neither tag is ever moved.

---

[← Docs index](../README.md#documentation)
