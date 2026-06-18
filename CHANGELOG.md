# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
