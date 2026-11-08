# Contributing to probaci

Where this file is silent, the [Simtabi contributing guide](https://github.com/simtabi/.github/blob/HEAD/CONTRIBUTING.md) applies.

Thanks for your interest in improving probaci.

## Development

```sh
git clone https://github.com/simtabi/probaci
cd probaci
make build            # build/<host-os>/probaci, version-stamped (or: task build)
make test             # go -C src test -race ./...
make vet
make lint             # golangci-lint, run in src/
```

The Go module lives in `src/`, so a bare `go test ./...` at the repository root
finds no packages and passes; name the module with `go -C src …`. `build/` holds
one folder per OS and nothing else, and `make check-build` fails on anything
else.

probaci dogfoods itself — once built, run it on the repo:

```sh
go -C src run ./cmd/probaci run --skip workflow-run
```

Cross-compile and validate the release config (none of these publishes):

```sh
make cross            # build/<os>/ binaries + build/checksums.txt
make dist             # the same, plus build/archives/
make release-check    # goreleaser check --config src/.goreleaser.yaml
```

`probaci docs` renders an embedded copy of `README.md` and `docs/` from
`src/embedded/`. After editing either, run `make sync-docs`; a test fails when
the copy drifts.

## Project layout

Everything Go lives in `src/`:

- `cmd/probaci` — entrypoint
- `internal/cli` — Cobra commands (thin; parse flags, drive the engine)
- `internal/stage` — the pipeline engine and stage implementations
- `internal/platform` — one adapter per CI service
- `internal/vcs` — version-control providers
- `internal/docker` — the least-privilege container broker
- `internal/tool` — the data-driven tool registry
- `internal/config`, `paths`, `logging`, `secret`, `ui`, `report`, `tui`,
  `run` (RunID), `discover` (repo-root walk-up)
- `pkg/probaci` — the public, embeddable API
- `internal/release` — tests guarding the release trigger, the `build/` layout,
  the absence of a container image, and the docs
- `scripts/install.sh` / `scripts/install.ps1` — end-user installers (keep asset
  names in sync with the GoReleaser archive `name_template`)

## Adding things

- **A CI platform**: implement `platform.Platform` and add it to `platform.All()`.
- **A tool**: add an entry to the registry in `internal/tool`, or let users add
  it via `tools.<name>` in config — no code needed.
- **A VCS provider**: implement `vcs.VCS` and register it in `internal/vcs`.

## Conventions

- Format with `gofmt`; keep `go vet` and `golangci-lint` clean.
- Commit subjects ≤ 72 chars, imperative mood; bodies explain *why*.
- Add tests for new logic, especially anything touching the broker, redaction,
  or config merging.

## Pull requests

Open against `main`. CI must pass. Describe the change and its motivation.
