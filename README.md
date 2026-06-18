# probaci

> **Prove your pipeline before you push.**

```
┌──────────────────────────────────────────────────────────┐
│                          probaci                          │
│              Prove your pipeline before you push          │
│        one binary · any VCS · any CI · every check        │
│                    in a container                         │
└──────────────────────────────────────────────────────────┘
```

**probaci** — from Latin *probāre*, "to test / to prove" (pronounced
**proh-BAH-see**). It *proves* your CI is green before you push: it runs the same
checks your CI runs, locally, with every tool brokered through a container, so a
red check surfaces on your machine in seconds instead of after a
commit–push–wait cycle. In short: it tests the tests.

It is a single, static, cross-platform Go binary. The only hard local
dependencies are a container runtime (Docker, rootless Docker, or Podman) and
your VCS — every CI tool and language toolchain runs in a pinned container, so
there is no "install N tools and hope their versions match CI."

## Why checks pass locally but fail in CI

probaci is built to catch the common causes automatically:

- **Uncommitted/untracked files** — your tests pass on files CI never sees. The
  `clean-clone` stage exports the *committed* state and tests that.
- **Dependency drift** — a locally-installed package that isn't in the lockfile.
  `clean-clone` installs from the lockfile in a clean container.
- **Runtime version mismatch** — the `versions` stage compares the runtime your
  workflow pins to what you have.
- **Workflow file bugs** — the `workflow-lint` stage statically checks them.
- **Runner-environment differences** — the `workflow-run` stage runs the real
  pipeline in containers.

## Install

Most users install the binary (you only need the source to change probaci):

```sh
# macOS / Linux — Homebrew
brew install simtabi/tap/probaci

# macOS / Linux — install script (downloads + checksum-verifies the right binary)
curl -fsSL https://raw.githubusercontent.com/simtabi/probaci/main/install.sh | sh

# Windows — Scoop, or the install script
scoop bucket add simtabi https://github.com/simtabi/scoop-bucket; scoop install probaci
irm https://raw.githubusercontent.com/simtabi/probaci/main/install.ps1 | iex

# Go toolchain
go install github.com/simtabi/probaci/cmd/probaci@latest
```

deb/rpm packages and a container image are published per release. Full details,
PATH guidance, and building from source: [docs/installation.md](docs/installation.md).

## Quickstart

```sh
probaci doctor              # runtime, detected languages/platforms/VCS
probaci init               # write a probaci.json for this repo
probaci run                # run the pipeline on the current directory
probaci run ./api ./web    # run on several repositories
probaci tui                # interactive dashboard
```

Positional arguments are **repository paths**; stage selection is via flags:

```sh
probaci run --only secrets,lint ./api ./web
probaci run --skip workflow-run
probaci run --repos ./api,./web --jobs 2     # comma form (CI/env friendly)
```

## The pipeline

Stages run cheapest-and-highest-signal first, so most problems surface before
the slow container workflow run:

`detect → workflow-lint → secrets → versions → lint → clean-clone → audit → sast
→ dockerfile-lint → yaml-lint → container-scan → commitlint → workflow-run`

Each stage is enabled/ordered in `probaci.json`. See
[docs/configuration.md](docs/configuration.md).

## Supported CI platforms

| Tier | Platforms |
|------|-----------|
| lint + local run | GitHub Actions, GitLab CI, Gitea/Forgejo, CircleCI, Drone, Woodpecker |
| lint / validate | Bitbucket, Azure Pipelines, Jenkins, Travis, Buildkite |

…across cloud, self-hosted, and enterprise endpoints. Adding a platform is an
adapter; adding a tool is a config entry.

## Version control

git, Mercurial (hg), Subversion (svn), Perforce/Helix (p4), Fossil,
Bazaar (bzr), Sapling (sl), and CVS (detect-only). `clean-clone` exports the
committed state on whichever you use.

## Configuration

probaci layers config: built-in defaults → `~/.config/probaci/config.json`
(user) → `./probaci.json` (project) → `PROBACI_*` env → CLI flags. The user home
is consistent across Linux/macOS/Windows and fully overridable. See
[docs/configuration.md](docs/configuration.md) and `probaci config --help`
(`path`, `show`, `init`, `edit`, `validate`, `reset`, `restore`, `migrate`).

## Security

Every brokered container runs least-privilege by default (non-root, all
capabilities dropped, no-new-privileges, read-only rootfs, offline network for
scanners, resource caps), images are pinned by digest, secrets are redacted from
all output and logs, and releases are signed with provenance + SBOM. Details in
[docs/security.md](docs/security.md) and [SECURITY.md](SECURITY.md).

<a id="documentation"></a>
## Documentation

| Page | What it covers |
|------|----------------|
| [Installation](docs/installation.md) | install methods, platforms, architectures |
| [Configuration](docs/configuration.md) | `probaci.json`, precedence, the config home |
| [Architecture](docs/architecture.md) | the broker model, packages, data flow |
| [Security](docs/security.md) | hardening defaults, the socket-mount caveat |
| [Release](docs/release.md) | tag-driven releases, signing, channels |
| [probaci tool](docs/tools/probaci.md) | command reference |

## License

MIT © Simtabi LLC. See [LICENSE](LICENSE).
