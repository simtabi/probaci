# Configuration

## Precedence

probaci merges configuration layers, lowest to highest:

```
built-in defaults
  → system config        (/etc/probaci/config.json, %ProgramData%\probaci\config.json)
  → ~/.config/probaci/config.json   (user-global)
  → ./probaci.json                  (per-project)
  → PROBACI_* environment
  → CLI flags
```

A missing file is never an error — probaci falls back to embedded defaults. The
**system layer** lets an admin set machine-wide defaults on a shared host while
each user's `~/.config/probaci` (and per-project config) override them. Override
the system path with `PROBACI_SYSTEM_DIR`.

## Single- vs multi-user & concurrency

- **Per-user isolation**: each user's config, logs, cache, and secrets live under
  their own home; no user can read or clobber another's state.
- **Concurrent runs are safe**: each invocation gets a `RunID`; logs are written
  per-run under `logs/<YYYY-MM-DD>/<HH-MM-SS>_<command>_<repo>_<runid>.log`
  (readable text, pruned after 14 days), so parallel processes never contend.
  Config-mutating commands take an advisory lock; writes are atomic.
- **Containers are labelled** (`probaci`, `probaci.user`, `probaci.run`);
  `probaci clean` reclaims only your leaked containers (`--all` for everyone).
- **Parallelism** across repositories is bounded by `--jobs`.

## The user home

Resolved consistently across OSes (and fully overridable):

```
$PROBACI_HOME  →  $XDG_CONFIG_HOME/probaci  →  ~/.config/probaci   (Linux/macOS)
                                              %AppData%\probaci     (Windows)
```

```
~/.config/probaci/
├── config.json          user-global config
├── config.schema.json   exported JSON Schema
├── tools.json           optional tool-registry overrides
├── logs/                rotating run logs
├── cache/               image/results cache
└── secrets/             0600 secret files (never committed)
```

Manage it with `probaci config`:

| Command | Effect |
|---------|--------|
| `config path` | print the resolved home and active files |
| `config show [--sources]` | print the effective merged config |
| `config init` | write the user-global config from defaults |
| `config edit` | open it in `$EDITOR` |
| `config validate` | validate against the schema |
| `config reset` | reset to factory defaults (backs up first) |
| `config restore [--from FILE]` | restore from the latest backup |
| `config migrate [file]` | import a legacy `ci-local.config.json` |

## probaci.json

```jsonc
{
  "version": 1,
  "members": [],                         // workspace globs; bare `run` expands them
  "project": {
    "languages": [], "install_cmd": "", "test_cmd": "", "lint_cmd": "", "audit_cmd": ""
  },
  "docker": {
    "enabled": true,
    "backend": "auto",                   // auto|docker|rootless|podman
    "pull": "missing",                   // missing|always|never
    "network": "",
    "resources": { "memory": "", "cpus": "", "pids_limit": 512 },
    "allow_socket_mount": false          // opt-in for nested-Docker runners
  },
  "security": {
    "verify_images": "advisory",         // advisory|strict
    "allow_unsigned": [],
    "registry_mirror": ""
  },
  "platforms": {
    "github": { "enabled": true, "base_url": "" },
    "gitlab": { "enabled": true, "base_url": "https://gitlab.example.com", "token_env": "GITLAB_TOKEN" }
  },
  "stages": [ { "name": "secrets", "enabled": true, "options": {} } ],
  "tools": {
    "gitleaks": { "image": "zricethezav/gitleaks", "tag": "v8.x", "digest": "sha256:…" }
  },
  "secrets_file": ".probaci/secrets",
  "env_file": ".env",
  "tui": { "theme": "auto" }
}
```

## Targeting repositories

Positional arguments are repository **paths**; stage selection is via flags.

```sh
probaci run ./api ./web        # space-separated paths (glob-friendly)
probaci run --repos ./api,./web # comma form for CI/env
probaci run -C ../service       # run as if started elsewhere
```

For monorepos, declare `members` globs in a root `probaci.json`; a bare
`probaci run` expands them.

---

[← Docs index](../README.md#documentation)
