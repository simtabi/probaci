# Installation

probaci is a single static binary. Two ways to get it, in this order: download
it from the GitHub release, or build from source. The GitHub release is the only
channel: this page offers no package-manager or `go install` path. The only hard
runtime dependencies are a container runtime (Docker, rootless Docker, or
Podman) and your VCS.

## Download a release

Each [GitHub release](https://github.com/simtabi/probaci/releases) carries bare
binaries `probaci_<os>_<arch>` (`.exe` on Windows; `macos`, not `darwin`),
archives, Linux `.deb`/`.rpm`/`.apk` packages, and `checksums.txt` (cosign-signed).
Download the one for your platform and the checksums, verify, and install:

```sh
gh release download --repo simtabi/probaci --pattern 'probaci_macos_universal' --pattern checksums.txt
shasum -a 256 -c checksums.txt --ignore-missing
chmod +x probaci_macos_universal && ./probaci_macos_universal install
```

Every asset in `checksums.txt` carries a build-provenance attestation:

```sh
gh attestation verify probaci_macos_universal --repo simtabi/probaci
```

## Build from source

You need Go (the version `src/go.mod` names), `git`, and `make` (or
[go-task](https://taskfile.dev)).

```sh
git clone https://github.com/simtabi/probaci ~/src/probaci
make -C ~/src/probaci build             # -> build/<os>/probaci (macos, linux or windows)
~/src/probaci/build/*/probaci install    # copies itself to ~/.local/bin (Windows: %LOCALAPPDATA%\Programs\probaci)
probaci doctor
```

`probaci install --system` installs machine-wide (`/usr/local/bin`,
`%ProgramFiles%`, needs elevation), `--dir DIR` picks an explicit directory, and
`probaci uninstall [--purge]` removes it again. Copying `build/<os>/probaci` onto
your `PATH` by hand works just as well. `<os>` is the host's OS, written `macos`
rather than `darwin`.

The Go module lives in `src/`, so without `make` the dependency-free primitive is
`go -C ~/src/probaci/src build -o ../build/linux/probaci ./cmd/probaci` (name the
folder for your OS). On Windows, run the same `git clone` and
`go -C src build -o ..\build\windows\probaci.exe ./cmd/probaci` from PowerShell
in the checkout, then run `build\windows\probaci.exe install`.

### Pinning a version

A version is a tag: check one out to build exactly that release (`git -C ~/src/probaci tag`
lists them).

```sh
git -C ~/src/probaci fetch --tags
git -C ~/src/probaci checkout vX.Y.Z
make -C ~/src/probaci build
```

### Updating

```sh
git -C ~/src/probaci pull --ff-only
make -C ~/src/probaci build
~/src/probaci/build/*/probaci install
```

`probaci version` reports the version the build was stamped with (`git describe`),
so a build from a branch is still identifiable.

### Other channels

The GitHub release is the only channel. The release workflow used to be set up
to publish a Homebrew cask, a Scoop manifest and a Winget manifest as well; no
probaci release ever reached them, and that publishing is gone from
`src/.goreleaser.yaml`. probaci builds no container image. `src/scripts/install.sh`
and `src/scripts/install.ps1` download release archives; see
[release.md](release.md).

## Run it globally

Once the install directory is on your `PATH`, run `probaci` from anywhere — it
auto-discovers the repository root by walking up from the current directory
(ddev-style), so `probaci run` works from any subdirectory.

If `probaci install` warns that the directory isn't on your `PATH`, add it:

| Shell / OS | Add to PATH |
|---|---|
| bash | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc` |
| zsh  | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc` |
| fish | `fish_add_path ~/.local/bin` |
| Windows (PowerShell) | `[Environment]::SetEnvironmentVariable('Path', "$dir;$env:Path", 'User')` |

`/usr/local/bin` is already on `PATH` for most systems — prefer it for a
machine-wide install (`probaci install --system`, which needs elevation).

## Supported platforms and architectures

| OS | Architectures |
|----|---------------|
| Linux | amd64, arm64, arm (v6/v7), 386 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64, 386 |

> The container broker requires a working container runtime. On 32-bit and some
> Windows configurations, runtime availability varies — probaci's `--no-docker`
> mode runs the non-container checks and skips the rest.

## Development builds

The Makefile is the one task definition; `task <name>` (Taskfile.yml) forwards
to it. The Go module lives in `src/`, and `build/` holds one folder per OS:

```sh
make build    # build/<host-os>/probaci, e.g. build/macos/probaci
make cross    # every target in build/targets.txt into build/<os>/, plus build/checksums.txt
make dist     # make cross, plus .tar.gz/.zip archives in build/archives/
go -C src build -o ../build/linux/probaci ./cmd/probaci   # the dependency-free primitive
```

Verify a cross build with `cd build && shasum -a 256 -c checksums.txt`. None of
these publishes anything. See [release.md](release.md).

---

[← Docs index](../README.md#documentation)
