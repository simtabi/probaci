# Installation

probaci ships as a single static binary. Most users should install the binary —
you only need the source to make changes. The only hard runtime dependencies are
a container runtime (Docker, rootless Docker, or Podman) and your VCS.

## Quick install (recommended)

**macOS / Linux — Homebrew:**

```sh
brew install simtabi/tap/probaci
```

**Windows — Scoop:**

```powershell
scoop bucket add simtabi https://github.com/simtabi/scoop-bucket
scoop install probaci
```

**macOS / Linux — install script** (downloads the right binary for your OS/arch,
verifies its checksum, installs it on your PATH):

```sh
curl -fsSL https://raw.githubusercontent.com/simtabi/probaci/main/install.sh | sh
```

**Windows — install script:**

```powershell
irm https://raw.githubusercontent.com/simtabi/probaci/main/install.ps1 | iex
```

The script installs to `/usr/local/bin` when it's writable or you're root,
otherwise `~/.local/bin` (Windows: `%LOCALAPPDATA%\Programs\probaci`, or
`%ProgramFiles%\probaci` when elevated). Override with `PROBACI_INSTALL_DIR`, and
pin a version with `PROBACI_VERSION`:

```sh
PROBACI_VERSION=0.1.0 PROBACI_INSTALL_DIR="$HOME/bin" \
  curl -fsSL https://raw.githubusercontent.com/simtabi/probaci/main/install.sh | sh
```

## Run it globally

Once the install directory is on your `PATH`, run `probaci` from anywhere — it
auto-discovers the repository root by walking up from the current directory
(ddev-style), so `probaci run` works from any subdirectory.

If the installer warns that the directory isn't on your `PATH`, add it:

| Shell / OS | Add to PATH |
|---|---|
| bash | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc` |
| zsh  | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc` |
| fish | `fish_add_path ~/.local/bin` |
| Windows (PowerShell) | `[Environment]::SetEnvironmentVariable('Path', "$dir;$env:Path", 'User')` |

`/usr/local/bin` is already on `PATH` for most systems — prefer it for a
machine-wide install (it may require `sudo` or `PROBACI_INSTALL_DIR=/usr/local/bin`
with elevation).

## go install (Go toolchain)

```sh
go install github.com/simtabi/probaci/cmd/probaci@latest
```

## Manual download

Download the archive for your OS/arch from
[GitHub Releases](https://github.com/simtabi/probaci/releases), verify it against
`checksums.txt` (and the cosign signature), extract, and either place `probaci`
on your `PATH` or let probaci place itself:

```sh
probaci install            # per-user (~/.local/bin, %LOCALAPPDATA%\Programs\probaci)
probaci install --system   # machine-wide (/usr/local/bin, %ProgramFiles%) — needs elevation
probaci install --dir DIR  # explicit directory
probaci uninstall [--purge]
```

## Container image

```sh
docker run --rm -v "$PWD:/workspace" ghcr.io/simtabi/probaci:latest run
```

## Supported platforms and architectures

| OS | Architectures |
|----|---------------|
| Linux | amd64, arm64, arm (v6/v7), 386 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64, 386 |

> The container broker requires a working container runtime. On 32-bit and some
> Windows configurations, runtime availability varies — probaci's `--no-docker`
> mode runs the non-container checks and skips the rest.

## Build from source (contributors only)

You only need this to modify probaci. One task definition, three entrypoints:

```sh
task build                               # go-task (Taskfile.yml)
make build                               # delegates to task, or falls back to go
go build -o bin/probaci ./cmd/probaci    # the dependency-free primitive
```

Produce local release deliverables (archives + checksums in `dist/`) with
`task bundle`. See [release.md](release.md).

---

[← Docs index](../README.md#documentation)
