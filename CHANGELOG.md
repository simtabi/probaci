# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Building probaci needs Go 1.26, and CI runs on it. Go 1.25 has ten standard-library
  vulnerabilities fixed only in 1.26, which the web UI's `net/http` server reaches; `golang.org/x/net`
  moves to v0.60.0 for the same reason, and gosec to v2.30.0.
- The release workflow no longer starts on a tag push. Binary releases are on hold, and a release
  runs only when dispatched by hand on a `v[0-9]*` tag with `-f confirm=release-go`; a test fails
  if an automatic trigger returns.
- The README and installation guide build probaci from source only: no download, install-script,
  package-manager or `go install` path is offered while binary releases are on hold.
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
  container image or docker step returns, and when the docs offer `go install`. The release
  trigger test now also requires the refusal to be the workflow's first step.
- A CI `snapshot` job that runs `make cross` on every pull request and uploads the binaries.

### Removed

- The container image. The `Dockerfile`, the GoReleaser `dockers_v2` block
  (`ghcr.io/simtabi/probaci`, linux/amd64 and linux/arm64), the release workflow's QEMU, buildx
  and ghcr.io login steps and its `packages: write` permission, and Dependabot's docker ecosystem
  are gone. No image was ever published, so nothing on ghcr.io is affected.

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
