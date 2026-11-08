# Architecture

probaci orchestrates checks; **the container broker is the universal runtime**.
The binary itself is small and pure-Go; every CI tool and language toolchain
runs inside a pinned container.

## Data flow

```
        ┌─────────┐   load + merge   ┌──────────┐
        │  CLI    │ ───────────────► │  config  │
        │  / TUI  │                  └────┬─────┘
        └────┬────┘                       │
             │ RunOptions                 ▼
             ▼                        ┌──────────┐
        ┌──────────┐   per repo       │  detect  │ languages
        │  engine  │ ───────────────► │   vcs    │ provider
        │ (stage)  │                  │ platform │ CI adapters
        └────┬─────┘                  └──────────┘
             │ each stage
             ▼
        ┌──────────┐   docker run --rm   ┌───────────────┐
        │  broker  │ ──────────────────► │ pinned tool   │
        │ (docker/ │   least-privilege   │ container     │
        │  podman) │                     └───────────────┘
        └────┬─────┘
             │ result.Result (+ streamed lines)
             ▼
        ┌──────────┐
        │  report  │  text / --json / TUI
        └──────────┘
```

## Packages

| Package | Responsibility |
|---------|----------------|
| `internal/cli` | Cobra commands; thin — parse flags, drive the engine |
| `internal/tui` | Bubble Tea dashboard subscribing to engine events |
| `internal/stage` | the pipeline engine and stage implementations |
| `internal/platform` | one adapter per CI service (tiered: run / lint / detect) |
| `internal/vcs` | version-control providers (git, hg, svn, p4, fossil, …) |
| `internal/docker` | the least-privilege container broker (docker/rootless/podman) |
| `internal/tool` | the data-driven tool registry |
| `internal/detect` | language detection and default commands |
| `internal/config` | schema, layered load/merge/validate, management ops |
| `internal/paths` | the cross-OS user home resolver |
| `internal/secret` | the central redactor |
| `internal/ui` / `report` | styling and human/JSON output |
| `pkg/probaci` | the public, embeddable API |

## Engine ↔ observers

The engine emits a structured `Event` stream as it runs. Both the CLI and the
TUI subscribe to the same stream, so there is exactly one runner and no
duplicated orchestration logic.

---

[← Docs index](../README.md#documentation)
