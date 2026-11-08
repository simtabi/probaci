# Installation

probaci is a single static binary, built from source. Binary releases are on
hold until the maintainer decides otherwise, so this page offers no download,
install-script, package-manager or `go install` path. The only hard runtime
dependencies are a container runtime (Docker, rootless Docker, or Podman) and
your VCS.

## Build from source

You need Go (the version `go.mod` names), `git`, and `make` (or
[go-task](https://taskfile.dev)).

```sh
git clone https://github.com/simtabi/probaci ~/src/probaci
make -C ~/src/probaci build       # -> bin/probaci
~/src/probaci/bin/probaci install  # copies itself to ~/.local/bin (Windows: %LOCALAPPDATA%\Programs\probaci)
probaci doctor
```

`probaci install --system` installs machine-wide (`/usr/local/bin`,
`%ProgramFiles%`, needs elevation), `--dir DIR` picks an explicit directory, and
`probaci uninstall [--purge]` removes it again. Copying `bin/probaci` onto your
`PATH` by hand works just as well.

Without `make`, the dependency-free primitive is
`go -C ~/src/probaci build -o bin/probaci ./cmd/probaci`. On Windows, run the same
`git clone` and that `go build` (with `-o bin\probaci.exe`) from PowerShell, then
run `bin\probaci.exe install`.

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
~/src/probaci/bin/probaci install
```

`probaci version` reports the version the build was stamped with (`git describe`),
so a build from a branch is still identifiable.

### Binary releases

No binary release has been published. The release tooling stays in the tree:
the GoReleaser config, and `scripts/install.sh` / `scripts/install.ps1`, which
download release archives. The release workflow would also publish Homebrew and
Scoop manifests, deb/rpm packages and a `ghcr.io/simtabi/probaci` container image.
None of those channels is offered while binary releases are on hold; see
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

One task definition, three entrypoints:

```sh
task build                               # go-task (Taskfile.yml)
make build                               # delegates to task, or falls back to go
go build -o bin/probaci ./cmd/probaci    # the dependency-free primitive
```

Produce local release deliverables (archives + checksums in `dist/`) with
`task bundle`. See [release.md](release.md).

---

[← Docs index](../README.md#documentation)
