# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `release.yml` calls simtabi/.github's shared `go-release.yml` at its `v0.1.0` release (`b3388da`), pinned as `@<commit>  # v0.1.0` so Dependabot can advance the commit and the tag comment together. v0.1.0 replaces the withdrawn `v1.0.0` and `v1.0.1`: it adds optional inputs only (`node-version` among them), fetches every owned org's Go modules from git rather than proxy.golang.org, and changes nothing a caller passes.
- `make check-build` asserts how many files it inspected in `build/`, not only that nothing is out of place: `make build` passes a floor of 2 (`targets.txt` and the binary) and `make cross` and `make dist` one per target plus `targets.txt` and `checksums.txt`, through `BUILD_LAYOUT_MIN`, so a check that reads an empty or wrong directory fails instead of passing.
- Building probaci needs Go 1.26, and CI runs on it. Go 1.25 has ten standard-library
  vulnerabilities fixed only in 1.26, which the web UI's `net/http` server reaches; `golang.org/x/net`
  moves to v0.60.0 for the same reason, and gosec to v2.30.0.
- Binaries ship on the GitHub release only. A release builds on a pushed `vX.Y.Z` tag (or a
  `vX.Y.Z-rc.N` pre-release), or on request for a tag already pushed
  (`gh workflow run release.yml --repo simtabi/probaci -f tag=vX.Y.Z`). `release.yml` is now a thin
  caller of simtabi/.github's shared `go-release.yml`, which checks the tag, takes the notes from
  this file, mirrors `src/vX.Y.Z`, runs GoReleaser and attests provenance. Go comes from a new
  `.go-version` (`1.26.11`), which every `setup-go` in `ci.yml` and `codeql.yml` now reads too, so
  CI, CodeQL and the release build on one exact toolchain rather than whatever 1.26 patch a runner
  has cached; 1.26.11 fixes GO-2026-6859 in `net/http`. This replaces the dispatch-only,
  `-f confirm=release-go` workflow of this cycle, which never shipped.
- The README and installation guide offer two paths, in order: download the binary from the GitHub
  release and check it against `checksums.txt`, or build from source. No package-manager or
  `go install` path is offered.
- The Go module moved into `src/`, and its path is now `github.com/simtabi/probaci/src`. Importers
  of `pkg/probaci` use `github.com/simtabi/probaci/src/pkg/probaci`. A release tags `src/vX.Y.Z`
  beside `vX.Y.Z` on the same commit, since that is the tag Go resolves the module through.
- `make build` writes `build/<os>/probaci` (macOS as `macos`) instead of `bin/probaci`.
  `make cross` and `make dist` write one folder per OS under `build/`, with
  `build/checksums.txt` over `{os}/{file}` paths and archives only in `build/archives/`;
  `build/dist/` is gone, and GoReleaser's scratch output moved to `.cache/goreleaser/`.
- The Makefile is the one task definition, and every Taskfile task forwards to it.
- `probaci docs` reads a copy of `README.md` and `docs/` kept in `src/embedded/` (`make sync-docs`),
  because the module in `src/` cannot embed files above it; a test fails when the copy drifts.
- The CI gitleaks scan runs a gitleaks v8.30.1 binary instead of the `zricethezav/gitleaks` image.

### Added

- `probaci serve`: a web UI on this machine, as an option beside `run` and `tui`. It lists the
  repositories and their languages, starts a run with the stages you pick, streams its output and
  stage results as they happen, can cancel it, and keeps every finished run in
  `$XDG_STATE_HOME/probaci/runs`. It follows the other Simtabi Go web UIs: a one-time sign-in
  token, a `Host` allowlist, same-origin and CSRF checks, a strict Content-Security-Policy, Tabler
  embedded with nothing fetched from the network, an SVG favicon on every page, and `--read-only`.
  `third-party-notices.md` carries the embedded licenses and ships in the archives.
- Tests that fail when `build/` holds anything but OS folders, inputs and checksums, when a
  container image or docker step returns, and when the docs offer `go install`.
- Tests that fail if `release.yml` gains a branch push, schedule, pull-request or release trigger,
  stops calling the shared release workflow, switches off the `src/vX.Y.Z` alias, or builds with a
  Go other than CI's, and if `src/.goreleaser.yaml` gains any publishing block besides the GitHub
  release.
- A CI `snapshot` job that runs `make cross` on every pull request and uploads the binaries.

### Removed

- The container image. The `Dockerfile`, the GoReleaser `dockers_v2` block
  (`ghcr.io/simtabi/probaci`, linux/amd64 and linux/arm64), the release workflow's QEMU, buildx
  and ghcr.io login steps and its `packages: write` permission, and Dependabot's docker ecosystem
  are gone. No image was ever published, so nothing on ghcr.io is affected.
- Homebrew cask, Scoop and Winget publishing from `src/.goreleaser.yaml`, and the tap and bucket
  deploy keys and token the release workflow passed it. No probaci release ever reached them, so
  nothing in `simtabi/homebrew-tap` or `simtabi/scoop-bucket` is affected.

### Fixed

- Language detection finds a Go module kept in `src/`, or in another directory one level down,
  instead of only a root `go.mod`. Install and test commands become `go -C <dir> …`, and the lint
  and audit containers start in that directory. `vendor/`, `testdata/`, `node_modules/` and
  directories starting with `.` or `_` are never taken for the module. probaci's own self-test
  reported `languages=[]` before this, and CI now asserts it detects Go.

## [0.1.0] - 2026-06-18

### Added

- Initial Go rewrite of the local-CI parity tool, renamed to **probaci**.
- Container broker with Docker, rootless Docker, and Podman backends; every tool
  runs least-privilege (non-root, cap-drop, no-new-privileges, read-only rootfs,
  offline-by-default network, resource caps).
- CI platform adapters: GitHub Actions, GitLab CI, Gitea/Forgejo, Bitbucket,
  CircleCI, Drone, Woodpecker (lint + run); Azure, Jenkins, Travis, Buildkite
  (lint/validate).
- Version-control providers: git, Mercurial, Subversion, Perforce, Fossil,
  Bazaar, Sapling, CVS (detect-only).
- Data-driven tool registry; languages auto-detected (Go, Python, Node, Rust,
  Ruby, Java).
- Layered configuration (`probaci.json`) with a cross-OS user home
  (`~/.config/probaci`), and `config` management (path/show/init/edit/validate/
  reset/restore/migrate, including legacy `ci-local.config.json` import).
- CLI plus a Bubble Tea TUI dashboard sharing one stage engine.
- Central secret redactor across stdout, `--json`, and the log file.
- Single binary with `Taskfile.yml` + passthrough `Makefile`; `install`/`uninstall`
  commands and ddev-style repo-root auto-discovery for global usage.
- Multi-user support: read-only system config layer (`/etc/probaci`,
  `%ProgramData%\probaci`) below per-user config; per-user isolation by default.
- Safe concurrent/parallel use: per-run, day-grouped, human-readable log files
  with retention; mutex-guarded event observer; atomic + flock-locked config
  writes; container ownership labels and a real `clean`.
- Image-trust policy: digest-pinning enforcement with `strict`/`advisory` modes,
  allow-list, and optional keyless cosign verification.
- Published JSON Schema (`config schema`, written on `config init`).
- `probaci docs [topic]` renders embedded documentation in the terminal (glamour),
  with a no-color style under `--ci`/`NO_COLOR`.
- `probaci init` is interactive on a TTY (huh form: stages, languages, backend),
  with a non-interactive fallback for `--ci`/`--yes`/piped input.
- `install.sh` / `install.ps1` one-liner installers (download, sha256-verify, and
  install the right release binary onto PATH); `task bundle`/`make bundle` build
  local release deliverables; `probaci tools --json` and `scripts/pin-digests.sh`
  to pin tool images by digest.
- Security gates: `.golangci.yml`, plus gosec, CodeQL, and OpenSSF Scorecard
  workflows.

### Fixed

- Config can now disable the container broker (`"docker":{"enabled":false}`);
  the previous merge ignored an explicit `false`. `Docker.Enabled` and
  `AllowSocketMount` are tri-state with `IsEnabled()`/`SocketMountAllowed()`.
- Per-repository config: multi-repo runs now load each repo's own `probaci.json`
  and tool registry instead of sharing one config.
- Readable log handler now renders attributes attached via `logger.With(...)`.
- clean-clone sets `HOME`/cache env so non-root container tools don't hit EACCES.
- The TUI cancels the engine on quit; the engine stops between stages on a
  canceled context.
- golangci-lint and gosec gates pass (with documented, justified exclusions for a
  CI orchestrator); the gosec action is pinned by commit SHA.

[Unreleased]: https://github.com/simtabi/probaci/commits/HEAD
