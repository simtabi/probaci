# Contributing to probaci

Thanks for your interest in improving probaci.

## Development

```sh
git clone https://github.com/simtabi/probaci
cd probaci
go build ./...        # or: task build / make build (version-stamped)
go test ./...
go vet ./...
golangci-lint run     # or: task lint
```

probaci dogfoods itself — once built, run it on the repo:

```sh
go run ./cmd/probaci run --skip workflow-run
```

Build local release deliverables and validate the release config:

```sh
task bundle           # dist/ archives + checksums (no publish)
task release-check    # goreleaser check (uses the pinned GoReleaser series)
```

## Project layout

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
- `install.sh` / `install.ps1` — end-user installers (keep asset names in sync
  with the GoReleaser archive `name_template`)

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
