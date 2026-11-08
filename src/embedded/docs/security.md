# Security

probaci executes third-party container images, runs CI tools over repository
content, and handles tokens. Hardening spans four layers. To report a
vulnerability, see [SECURITY.md](../SECURITY.md).

## 1. Command execution — no shell

The original bash tool used `eval`; probaci eliminates that class entirely.
Tools run via `exec` with **argument vectors only** — config values are never
concatenated into a shell string. Config is schema-validated (unknown fields
rejected), and target paths are resolved and confined to the repository root.

## 2. Least-privilege containers

Every brokered run defaults to:

```
--rm --cap-drop ALL --security-opt no-new-privileges
--read-only --tmpfs /tmp        (writable scratch only)
--user <host uid:gid>           (non-root, on Linux)
--pids-limit / --memory / --cpus
--network none                  (offline unless the tool needs the network)
-v <repo>:/workspace:ro         (read-only repo; clean-clone uses an isolated rw export)
```

Backends: Docker (default), **rootless Docker**, and **Podman** — choose in
config or `--backend`.

### The socket-mount caveat

Nested-Docker runners (`act`, `gitlab-ci-local`) may need the container socket.
probaci **never** mounts it for its own tool runs; enabling it for those runners
is opt-in (`docker.allow_socket_mount`) and is best done on the rootless/Podman
backend.

## 3. Secrets

- Passed via env file or stdin, **never argv**.
- Secret files live `0600` under `~/.config/probaci/secrets/` and are
  git-ignored by default.
- A central **redactor** scrubs known secret values and token-shaped strings
  from stdout, `--json`, and the rotating log file.

## 4. Image trust

- Images are referenced by **digest** where pinned (tamper-evident).
- **cosign verification is advisory by default** (verify + warn); `strict` mode
  refuses unverified images unless allow-listed.
- A private `registry_mirror` can be configured.

## Supply chain (releases)

Releases are reproducible and carry SHA-256 checksums, **cosign** signatures,
an **SBOM**, and **SLSA provenance**, published via OIDC trusted publishing. CI
runs `govulncheck`, `golangci-lint`, and `gitleaks`, with all Actions pinned by
commit SHA and `permissions: contents: read` by default.

---

[← Docs index](../README.md#documentation)
