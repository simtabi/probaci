# probaci — command reference

```
probaci [command] [flags] [PATH...]
```

## Commands

| Command | Description |
|---------|-------------|
| `run [PATH...]` | run the pipeline on one or more repositories (default: `.`) |
| `stage <name> [PATH...]` | run a single stage |
| `tui [PATH...]` | launch the interactive dashboard |
| `doctor [PATH]` | runtime, detected platforms/languages/VCS, config provenance |
| `init [PATH]` | write a `probaci.json` detected from the repo |
| `platforms [PATH]` | list supported CI platforms and which are detected |
| `vcs [PATH]` | show the detected version-control provider(s) |
| `tools` | list the tool registry and resolved images |
| `docs [topic]` | documentation pointers |
| `logs [--self]` | probaci's own log location (or remote-CI failures) |
| `clean` | prune probaci-labelled container artifacts |
| `config …` | path / show / init / edit / validate / reset / restore / migrate |
| `version` | print version information |

## Global flags

| Flag | Description |
|------|-------------|
| `--only a,b` / `--skip a,b` | stage selection (comma-separated) |
| `--repos a,b` | comma-separated repo paths (alternative to positionals) |
| `-C, --chdir DIR` | run as if started in DIR |
| `--jobs N` | repositories to process concurrently |
| `--platform NAME` | restrict to one CI platform |
| `--config FILE` | override the project config file |
| `--backend` | `auto\|docker\|rootless\|podman` |
| `--pull` | `missing\|always\|never` |
| `--no-docker` | disable the broker (degraded mode) |
| `--full-image` | heavy runner image for `workflow-run` |
| `--dry-run` | print the plan without executing |
| `--ci` | non-interactive, plain output |
| `--json` | machine-readable output |
| `-v, --verbose` | increase log verbosity (repeatable) |

## Stages

`detect`, `workflow-lint`, `secrets`, `versions`, `lint`, `clean-clone`,
`audit`, `sast`, `dockerfile-lint`, `yaml-lint`, `container-scan`, `commitlint`,
`workflow-run`.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | all clear |
| 1 | a stage failed |
| 2 | usage/config error |
| 125 | container runtime unavailable |

---

[← Docs index](../../README.md#documentation)
